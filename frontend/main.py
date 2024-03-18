import streamlit as st
import requests
from typing import List
from PIL import Image
import io
import base64

# 获取模型列表
@st.cache_data
def get_models() -> List[dict]:
    response = requests.get('/api/models')
    return response.json()['models']

# 发送请求并获取响应
def process_request(model_id, content, file):
    data = {'content': content, 'model': model_id}
    files = {}
    if file is not None:
        files['file'] = file
    response = requests.post('/api/process', data=data, files=files)
    return response.json()

def main():
    st.set_page_config(layout="wide")

    # 顶部模型选择
    models = get_models()
    model_id = st.sidebar.selectbox('选择模型', [model['id'] for model in models], format_func=lambda id: next((model['name'] for model in models if model['id'] == id), id))

    # 中间展示区域
    content_placeholder = st.empty()

    # 输入栏和控制按钮
    input_col, controls_col = st.columns([3, 1])
    with input_col:
        user_input = st.text_area('输入内容')
    with controls_col:
        uploaded_file = st.file_uploader('上传文件', type=['png', 'jpg', 'jpeg', 'gif', 'md'])
        submit_button = st.button('发送')

    # 处理提交
    if submit_button:
        content = user_input
        file = uploaded_file if uploaded_file is not None else None
        response = process_request(model_id, content, file)

        # 清空输入栏和文件上传区域
        user_input = ''
        uploaded_file = None

        # 展示响应内容
        if response['type'] == 'text':
            content_placeholder.write(response['content'])
        elif response['type'] == 'markdown':
            content_placeholder.markdown(response['content'])
        elif response['type'] == 'image':
            image_bytes = base64.b64decode(response['content'])
            image = Image.open(io.BytesIO(image_bytes))
            content_placeholder.image(image)
        elif response['type'] == 'url':
            url_response = requests.get(response['content']).json()
            if url_response['type'] == 'text':
                content_placeholder.write(url_response['content'])
            elif url_response['type'] == 'markdown':
                content_placeholder.markdown(url_response['content'])
            elif url_response['type'] == 'image':
                image_bytes = base64.b64decode(url_response['content'])
                image = Image.open(io.BytesIO(image_bytes))
                content_placeholder.image(image)

if __name__ == '__main__':
    main()
