import ctypes
import sys
import os
import threading
import time


# Create a dictionary to store callback data for each request
callback_data_store = {}

PROMPT_TEXT_PREFIX = "<|im_start|>system You are a helpful assistant. <|im_end|> <|im_start|>user"
PROMPT_TEXT_POSTFIX = "<|im_end|><|im_start|>assistant"

# Define the structures from the library
RKLLM_Handle_t = ctypes.c_void_p
userdata = ctypes.c_void_p(None)

LLMCallState = ctypes.c_int
LLMCallState.RKLLM_RUN_NORMAL = 0
LLMCallState.RKLLM_RUN_WAITING = 1
LLMCallState.RKLLM_RUN_FINISH = 2
LLMCallState.RKLLM_RUN_ERROR = 3
LLMCallState.RKLLM_RUN_GET_LAST_HIDDEN_LAYER = 4

RKLLMInputMode = ctypes.c_int
RKLLMInputMode.RKLLM_INPUT_PROMPT = 0
RKLLMInputMode.RKLLM_INPUT_TOKEN = 1
RKLLMInputMode.RKLLM_INPUT_EMBED = 2
RKLLMInputMode.RKLLM_INPUT_MULTIMODAL = 3

RKLLMInferMode = ctypes.c_int
RKLLMInferMode.RKLLM_INFER_GENERATE = 0
RKLLMInferMode.RKLLM_INFER_GET_LAST_HIDDEN_LAYER = 1

class RKLLMExtendParam(ctypes.Structure):
    _fields_ = [
        ("base_domain_id", ctypes.c_int32),
        ("reserved", ctypes.c_uint8 * 112)
    ]

class RKLLMParam(ctypes.Structure):
    _fields_ = [
        ("model_path", ctypes.c_char_p),
        ("max_context_len", ctypes.c_int32),
        ("max_new_tokens", ctypes.c_int32),
        ("top_k", ctypes.c_int32),
        ("top_p", ctypes.c_float),
        ("temperature", ctypes.c_float),
        ("repeat_penalty", ctypes.c_float),
        ("frequency_penalty", ctypes.c_float),
        ("presence_penalty", ctypes.c_float),
        ("mirostat", ctypes.c_int32),
        ("mirostat_tau", ctypes.c_float),
        ("mirostat_eta", ctypes.c_float),
        ("skip_special_token", ctypes.c_bool),
        ("is_async", ctypes.c_bool),
        ("img_start", ctypes.c_char_p),
        ("img_end", ctypes.c_char_p),
        ("img_content", ctypes.c_char_p),
        ("extend_param", RKLLMExtendParam),
    ]

class RKLLMLoraAdapter(ctypes.Structure):
    _fields_ = [
        ("lora_adapter_path", ctypes.c_char_p),
        ("lora_adapter_name", ctypes.c_char_p),
        ("scale", ctypes.c_float)
    ]

class RKLLMEmbedInput(ctypes.Structure):
    _fields_ = [
        ("embed", ctypes.POINTER(ctypes.c_float)),
        ("n_tokens", ctypes.c_size_t)
    ]

class RKLLMTokenInput(ctypes.Structure):
    _fields_ = [
        ("input_ids", ctypes.POINTER(ctypes.c_int32)),
        ("n_tokens", ctypes.c_size_t)
    ]

class RKLLMMultiModelInput(ctypes.Structure):
    _fields_ = [
        ("prompt", ctypes.c_char_p),
        ("image_embed", ctypes.POINTER(ctypes.c_float)),
        ("n_image_tokens", ctypes.c_size_t)
    ]

class RKLLMInputUnion(ctypes.Union):
    _fields_ = [
        ("prompt_input", ctypes.c_char_p),
        ("embed_input", RKLLMEmbedInput),
        ("token_input", RKLLMTokenInput),
        ("multimodal_input", RKLLMMultiModelInput)
    ]

class RKLLMInput(ctypes.Structure):
    _fields_ = [
        ("input_mode", ctypes.c_int),
        ("input_data", RKLLMInputUnion)
    ]

class RKLLMLoraParam(ctypes.Structure):
    _fields_ = [
        ("lora_adapter_name", ctypes.c_char_p)
    ]

class RKLLMPromptCacheParam(ctypes.Structure):
    _fields_ = [
        ("save_prompt_cache", ctypes.c_int),
        ("prompt_cache_path", ctypes.c_char_p)
    ]

class RKLLMInferParam(ctypes.Structure):
    _fields_ = [
        ("mode", RKLLMInferMode),
        ("lora_params", ctypes.POINTER(RKLLMLoraParam)),
        ("prompt_cache_params", ctypes.POINTER(RKLLMPromptCacheParam))
    ]

class RKLLMResultLastHiddenLayer(ctypes.Structure):
    _fields_ = [
        ("hidden_states", ctypes.POINTER(ctypes.c_float)),
        ("embd_size", ctypes.c_int),
        ("num_tokens", ctypes.c_int)
    ]

class RKLLMResult(ctypes.Structure):
    _fields_ = [
        ("text", ctypes.c_char_p),
        ("size", ctypes.c_int),
        ("last_hidden_layer", RKLLMResultLastHiddenLayer)
    ]

# Define a thread-local storage for callback data
class CallbackData:
    def __init__(self):
        self.text = []
        self.state = -1
        self.split_byte_data = bytes(b"")


# Connect the callback function between the Python side and the C++ side
callback_type = ctypes.CFUNCTYPE(None, ctypes.POINTER(RKLLMResult), ctypes.c_void_p, ctypes.c_int)

# Define the callback function with request_id parameter
def create_callback(request_id):
    def callback_impl(result, userdata, state):
        if request_id not in callback_data_store:
            callback_data_store[request_id] = CallbackData()
        
        data = callback_data_store[request_id]
        
        if state == LLMCallState.RKLLM_RUN_FINISH:
            data.state = state
        elif state == LLMCallState.RKLLM_RUN_ERROR:
            data.state = state
            print(f"[Request {request_id}] Run error")
            sys.stdout.flush()
        elif state == LLMCallState.RKLLM_RUN_GET_LAST_HIDDEN_LAYER:
            # Handle hidden layer data
            if result.last_hidden_layer.embd_size != 0 and result.last_hidden_layer.num_tokens != 0:
                data_size = result.last_hidden_layer.embd_size * result.last_hidden_layer.num_tokens * ctypes.sizeof(ctypes.c_float)
                # print(f"[Request {request_id}] data_size: {data_size}")
                data.text.append(f"data_size: {data_size}\n")
                output_path = os.getcwd() + f"/last_hidden_layer_{request_id}.bin"
                with open(output_path, "wb") as outFile:
                    data_ptr = ctypes.cast(result.last_hidden_layer.hidden_states, ctypes.POINTER(ctypes.c_float))
                    float_array_type = ctypes.c_float * (data_size // ctypes.sizeof(ctypes.c_float))
                    float_array = float_array_type.from_address(ctypes.addressof(data_ptr.contents))
                    outFile.write(bytearray(float_array))
                    print(f"[Request {request_id}] Data saved to {output_path} successfully!")
                    data.text.append(f"Data saved to {output_path} successfully!")
            else:
                data.text.append("Invalid hidden layer data.")
            
            data.state = state
            time.sleep(0.05)  # Delay for 0.05 seconds to wait for the output result
        else:
            # Save the output token text and the RKLLM running state
            data.state = state
            # Monitor if the current byte data is complete; if incomplete, record it for later parsing
            try:
                data.text.append((data.split_byte_data + result.contents.text).decode('utf-8'))
                data.split_byte_data = bytes(b"")
            except:
                data.split_byte_data += result.contents.text
    
    return callback_impl


# 修改RKLLM类以支持多请求使用同一模型实例
class RKLLM(object):
    def __init__(self, library_path, model_path, request_id, lora_model_path=None, prompt_cache_path=None):
        
        self.rkllm_lib = ctypes.CDLL(library_path)

        rkllm_param = RKLLMParam()
        rkllm_param.model_path = bytes(model_path, 'utf-8')

        rkllm_param.max_context_len = 512
        rkllm_param.max_new_tokens = -1
        rkllm_param.skip_special_token = True

        rkllm_param.top_k = 1
        rkllm_param.top_p = 0.9
        rkllm_param.temperature = 0.8
        rkllm_param.repeat_penalty = 1.1
        rkllm_param.frequency_penalty = 0.0
        rkllm_param.presence_penalty = 0.0

        rkllm_param.mirostat = 0
        rkllm_param.mirostat_tau = 5.0
        rkllm_param.mirostat_eta = 0.1

        rkllm_param.is_async = False

        rkllm_param.img_start = "".encode('utf-8')
        rkllm_param.img_end = "".encode('utf-8')
        rkllm_param.img_content = "".encode('utf-8')

        rkllm_param.extend_param.base_domain_id = 0
        
        self.handle = RKLLM_Handle_t()

        # 存储每个请求ID对应的回调函数
        self.callbacks = {}
        
        self.rkllm_init = self.rkllm_lib.rkllm_init
        self.rkllm_init.argtypes = [ctypes.POINTER(RKLLM_Handle_t), ctypes.POINTER(RKLLMParam), callback_type]
        self.rkllm_init.restype = ctypes.c_int
        
        self.global_callback = callback_type(self.global_callback_handler)
        ret = self.rkllm_init(ctypes.byref(self.handle), ctypes.byref(rkllm_param), self.global_callback)
        print(f"RKLLM init ret: {ret}")
            #raise Exception("Failed to initialize RKLLM")

        self.rkllm_run = self.rkllm_lib.rkllm_run
        self.rkllm_run.argtypes = [RKLLM_Handle_t, ctypes.POINTER(RKLLMInput), ctypes.POINTER(RKLLMInferParam), ctypes.c_void_p]
        self.rkllm_run.restype = ctypes.c_int

        self.rkllm_destroy = self.rkllm_lib.rkllm_destroy
        self.rkllm_destroy.argtypes = [RKLLM_Handle_t]
        self.rkllm_destroy.restype = ctypes.c_int

        # 处理LoRA模型（如果提供）
        self.lora_adapter_path = None
        self.lora_model_name = None
        if lora_model_path:
            self.lora_adapter_path = lora_model_path
            self.lora_adapter_name = "test"

            lora_adapter = RKLLMLoraAdapter()
            ctypes.memset(ctypes.byref(lora_adapter), 0, ctypes.sizeof(RKLLMLoraAdapter))
            lora_adapter.lora_adapter_path = ctypes.c_char_p((self.lora_adapter_path).encode('utf-8'))
            lora_adapter.lora_adapter_name = ctypes.c_char_p((self.lora_adapter_name).encode('utf-8'))
            lora_adapter.scale = 1.0

            rkllm_load_lora = self.rkllm_lib.rkllm_load_lora
            rkllm_load_lora.argtypes = [RKLLM_Handle_t, ctypes.POINTER(RKLLMLoraAdapter)]
            rkllm_load_lora.restype = ctypes.c_int
            rkllm_load_lora(self.handle, ctypes.byref(lora_adapter))
        
        # 处理提示缓存（如果提供）
        self.prompt_cache_path = None
        if prompt_cache_path:
            self.prompt_cache_path = prompt_cache_path

            rkllm_load_prompt_cache = self.rkllm_lib.rkllm_load_prompt_cache
            rkllm_load_prompt_cache.argtypes = [RKLLM_Handle_t, ctypes.c_char_p]
            rkllm_load_prompt_cache.restype = ctypes.c_int
            rkllm_load_prompt_cache(self.handle, ctypes.c_char_p((prompt_cache_path).encode('utf-8')))
    
    # 根据当前请求ID路由到正确的回调处理
    def global_callback_handler(self, result, userdata, state):
        # 获取当前活动请求ID（通过线程本地存储或上下文管理设置）
        current_request_id = getattr(threading.current_thread(), 'request_id', "global")
        
        # 调用相应请求ID的回调函数
        if current_request_id in callback_data_store:
            callback_impl = create_callback(current_request_id)
            callback_impl(result, userdata, state)
    
    # 执行LLM推理
    def run(self, prompt, request_id):
        # 将请求ID绑定到当前线程
        threading.current_thread().request_id = request_id
        
        rkllm_lora_params = None
        if self.lora_model_name:
            rkllm_lora_params = RKLLMLoraParam()
            rkllm_lora_params.lora_adapter_name = ctypes.c_char_p((self.lora_model_name).encode('utf-8'))
        
        rkllm_infer_params = RKLLMInferParam()
        ctypes.memset(ctypes.byref(rkllm_infer_params), 0, ctypes.sizeof(RKLLMInferParam))
        rkllm_infer_params.mode = RKLLMInferMode.RKLLM_INFER_GENERATE
        rkllm_infer_params.lora_params = ctypes.byref(rkllm_lora_params) if rkllm_lora_params else None

        rkllm_input = RKLLMInput()
        rkllm_input.input_mode = RKLLMInputMode.RKLLM_INPUT_PROMPT
        rkllm_input.input_data.prompt_input = ctypes.c_char_p((PROMPT_TEXT_PREFIX + prompt + PROMPT_TEXT_POSTFIX).encode('utf-8'))
        self.rkllm_run(self.handle, ctypes.byref(rkllm_input), ctypes.byref(rkllm_infer_params), None)
        return

    def release(self):
        self.rkllm_destroy(self.handle)
