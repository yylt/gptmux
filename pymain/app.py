import sys
import os
import subprocess
import resource
import threading
import time
import json
from typing import List, Dict, Optional, Any, Generator
import asyncio
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException, Request, BackgroundTasks
from fastapi.responses import JSONResponse, StreamingResponse
from starlette.middleware.base import BaseHTTPMiddleware
from pydantic import BaseModel
import uvicorn
from rkllm import llm

global_rkllm_model = None

# Global model configuration
model_config = {
    'library_path': None,
    'model_path': None,
    'target_platform': None,
    'lora_model_path': None,
    'prompt_cache_path': None
}

# Models for request validation
class Message(BaseModel):
    role: str
    content: str

class ChatRequest(BaseModel):
    messages: List[Message]
    stream: Optional[bool] = False

@asynccontextmanager
async def lifespan(app: FastAPI):
    # 在服务启动时初始化模型
    global global_rkllm_model
    
    # 设置资源限制
    resource.setrlimit(resource.RLIMIT_NOFILE, (102400, 102400))
    
    # 初始化全局模型实例
    print("Initializing global RKLLM model...")
    global_rkllm_model = llm.RKLLM(
        model_config['library_path'],
        model_config['model_path'],
        "global",  # 使用"global"作为全局模型的请求ID
        model_config['lora_model_path'],
        model_config['prompt_cache_path']
    )
    print("Global RKLLM model initialized successfully")

    yield

    # Clean up, 在服务关闭时释放模型资源 
    if global_rkllm_model:
        print("Releasing global RKLLM model resources...")
        global_rkllm_model.release()
        global_rkllm_model = None
        print("Global RKLLM model resources released successfully")

async def cleanup_task(request_id: str):
    global_rkllm_model.stop()
    llm.callback_data_store.pop(request_id, None)

app = FastAPI(lifespan=lifespan)
lock = asyncio.Lock()

# 修改路由处理函数，使用全局模型实例
@app.post("/rkllm_chat")
async def rkllm_chat(chat_request: ChatRequest, background_tasks: BackgroundTasks, request: Request):
    global global_rkllm_model
    global lock
    if lock.locked():
        raise HTTPException(status_code=500, detail="资源繁忙，请稍后重试")

    async with lock:
        request_id = str(id(request))
        
        llm.callback_data_store[request_id] = llm.CallbackData()
        background_tasks.add_task(cleanup_task, request_id)
        
        now = int(time.time())
        
        # only streaming
        async def generate_stream():
            for index, message in enumerate(chat_request.messages):
                input_prompt = message.content
                
                model_thread = threading.Thread(
                    target=global_rkllm_model.run, 
                    args=(input_prompt, request_id)
                )
                model_thread.start()
                
                model_thread_finished = False
                while not model_thread_finished:
                    while len(llm.callback_data_store[request_id].text) > 0:
                        rkllm_output = llm.callback_data_store[request_id].text.pop(0)
                        
                        stream_response = {
                            "id": "rkllm_chat",
                            "object": "rkllm_chat",
                            "created": now,
                            "choices": [{
                                "index": index,
                                "delta": {
                                    "role": "assistant",
                                    "content": rkllm_output,
                                },
                                "finish_reason": "stop" if llm.callback_data_store[request_id].state == llm.LLMCallState.RKLLM_RUN_FINISH else None,
                            }]
                        }
                        
                        yield f"data: {json.dumps(stream_response, ensure_ascii=False)}\n\n"
                    
                    await asyncio.sleep(0.005)
                    model_thread_finished = not model_thread.is_alive()
                
                # 完成
                yield f"data: [DONE]\n\n"
        
        return StreamingResponse(
            generate_stream(),
            media_type="text/event-stream"
        )


def start_server(library_path, model_path, target_platform, lora_model_path=None, prompt_cache_path=None, host="127.0.0.1", port=8080):
    # Validate
    if not os.path.exists(library_path):
        print("Error: Please provide the correct library path, and ensure it is the absolute path on the board.")
        sys.exit(1)

    if not os.path.exists(model_path):
        print("Error: Please provide the correct rkllm model path, and ensure it is the absolute path on the board.")
        sys.exit(1)
    
    # Validate target platform
    if target_platform not in ["rk3588", "rk3576"]:
        print("Error: Please specify the correct target platform: rk3588/rk3576.")
        sys.exit(1)
    
    # Validate lora model path if provided
    if lora_model_path and not os.path.exists(lora_model_path):
        print("Error: Please provide the correct lora_model path, and ensure it is the absolute path on the board.")
        sys.exit(1)
    
    # Validate prompt cache path if provided
    if prompt_cache_path and not os.path.exists(prompt_cache_path):
        print("Error: Please provide the correct prompt_cache_file path, and ensure it is the absolute path on the board.")
        sys.exit(1)
    
    # Set global model configuration
    model_config['library_path'] = library_path
    model_config['model_path'] = model_path
    model_config['target_platform'] = target_platform
    model_config['lora_model_path'] = lora_model_path
    model_config['prompt_cache_path'] = prompt_cache_path
    
    print("==================================")
    print("Starting RKLLM FastAPI Server...")
    print(f"Model path: {model_path}")
    print(f"Target platform: {target_platform}")
    if lora_model_path:
        print(f"LoRA model path: {lora_model_path}")
    if prompt_cache_path:
        print(f"Prompt cache path: {prompt_cache_path}")
    print("==================================")
    
    # Start the server
    uvicorn.run(app, host=host, port=port, log_level="info")

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="RKLLM FastAPI Server")
    parser.add_argument('--model-path', type=str, required=True, help='Absolute path of the converted RKLLM model on the Linux board')
    parser.add_argument('--library-path', type=str, required=True, help='Absolute path of the library "librkllmrt.so" path')
    parser.add_argument('--target-platform', type=str, default="rk3588", help='Target platform: e.g., rk3588/rk3576')
    parser.add_argument('--lora-model-path', type=str, help='Absolute path of the lora_model on the Linux board')
    parser.add_argument('--prompt-cache-path', type=str, help='Absolute path of the prompt_cache file on the Linux board')
    parser.add_argument('--host', type=str, default="0.0.0.0", help='Host to bind the server to')
    parser.add_argument('--port', type=int, default=8080, help='Port to bind the server to')
    
    args = parser.parse_args()
    
    start_server(
        args.library_path,
        args.model_path,
        args.target_platform,
        args.lora_model_path,
        args.prompt_cache_path,
        args.host,
        args.port
    )
