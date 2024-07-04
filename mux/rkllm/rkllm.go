package rkllm

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tmc/langchaingo/llms"
	"github.com/yylt/gptmux/mux"
	"github.com/yylt/gptmux/pkg"
	"k8s.io/klog/v2"
)

type Conf struct {
	Lib       string `yaml:"lib"`
	ModelPath string `yaml:"model_path"`
	Index     int    `yaml:"index,omitempty"`
}
type rkllm struct {
	c  *Conf
	mu sync.Mutex
}

type token struct {
	err     error
	done    bool
	content string
}
type result struct {
	Text   string
	tokens uintptr
	num    int
}

type param struct {
	model_path        string
	num_npu_core      int32
	max_context_len   int32
	max_new_tokens    int32
	top_k             int32
	top_p             float32
	temperature       float32
	repeat_penalty    float32
	frequency_penalty float32
	presence_penalty  float32
	mirostat          int32
	mirostat_tau      float32
	mirostat_eta      float32
	logprobs          bool
	top_logprobs      int32
	use_gpu           bool
}

var (
	rkllm_init func(uintptr, unsafe.Pointer, uintptr) int
	rkllm_run  func(uintptr, string, unsafe.Pointer) int

	rkllm_destroy func(uintptr) int
	rkllm_abort   func(uintptr) int

	// must global
	voidfn     = purego.NewCallback(void)
	callbackfn = purego.NewCallback(callback)

	tokench = make(chan *token, 128)
)

func New(c *Conf) *rkllm {
	mpath := c.ModelPath
	lib := c.Lib

	if lib == "" || mpath == "" {
		klog.Warningf("rkllm init failed: invalid config")
		return nil
	}
	libc, err := purego.Dlopen(lib, purego.RTLD_DEFAULT|purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	// binding to library
	purego.RegisterLibFunc(&rkllm_init, libc, "rkllm_init")
	purego.RegisterLibFunc(&rkllm_destroy, libc, "rkllm_destroy")
	purego.RegisterLibFunc(&rkllm_run, libc, "rkllm_run")
	purego.RegisterLibFunc(&rkllm_abort, libc, "rkllm_abort")

	pa := &param{
		model_path:        mpath,
		num_npu_core:      3,
		max_context_len:   4096,
		max_new_tokens:    10240,
		top_k:             3,
		top_p:             0.9,
		temperature:       0.8,
		repeat_penalty:    1.1,
		frequency_penalty: 0.0,
		presence_penalty:  0.0,
		mirostat:          0,
		mirostat_tau:      5.0,
		mirostat_eta:      0.1,
		logprobs:          false,
		top_logprobs:      5,
		use_gpu:           true,
	}
	ret := rkllm_init(voidfn, unsafe.Pointer(pa), callbackfn)
	if ret != 0 {
		klog.Exitf("rkllm init failed: %v\n", ret)
	}
	klog.Infof("rkllm init success")

	return &rkllm{c: c}
}

func (d *rkllm) Name() string {
	return "rkllm"
}

func (d *rkllm) Index() int {
	return d.c.Index
}

func (d *rkllm) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if !d.mu.TryLock() {
		return nil, pkg.BusyErr
	}
	defer d.mu.Unlock()
	prompt, model := mux.GeneraPrompt(messages)

	if model != pkg.TxtModel {
		return nil, fmt.Errorf("not support model '%s'", model)
	}
	ret := rkllm_run(voidfn, prompt, unsafe.Pointer(nil))
	if ret != 0 {
		return nil, fmt.Errorf("run failed, exit code: %v", ret)
	}

	var (
		opt          = &llms.CallOptions{}
		bctx, cancle = context.WithCancel(context.Background())
		data         = &llms.ContentResponse{}
	)
	for _, o := range options {
		o(opt)
	}
	defer func() {
		cancle()
		opt.StreamingFunc(bctx, nil)
	}()

	for v := range tokench {
		if v.err != nil {
			return nil, fmt.Errorf("run failed: %v", v.err)
		}
		data.Choices = append(data.Choices, &llms.ContentChoice{
			Content: v.content,
		})

		if opt.StreamingFunc != nil {
			err := opt.StreamingFunc(bctx, []byte(v.content))
			if err != nil {
				return data, nil
			}
		}
		if v.done {
			break
		}
	}
	return data, nil
}

func (d *rkllm) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", fmt.Errorf("not implement")
}

func callback(r *result, a uintptr, state int) {

	if r != nil {
		klog.Infof("result %v, text: %s, state: %d\n", r, r.Text, state)
	} else {
		klog.Infof("result is null, %v\n", r)
		tokench <- &token{
			err: fmt.Errorf("null result"),
		}
		return
	}

	tokench <- &token{
		content: r.Text,
		done:    state == 1,
	}
}

func void() {}
