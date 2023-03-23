package serve

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
}

type Speaker string

const (
	AiSpeaker    Speaker = "ai"
	HumanSpeaker Speaker = "human"
)

type ListResp struct {
	Id     string   `json:"_id"`
	Title  string   `json:"title"`
	Conves []conver `json:"convs"`
}

type conver struct {
	Speaker  Speaker  `json:"speaker"`
	Speech   string   `json:"speech"`
	speeches []string `json:"speeches"`
}

func NewGeneralResp(o int, data interface{}) *Response {
	return &Response{
		Code: o,
		Data: data,
	}
}

func NewListResp(id string, title string, kvs ...string) *ListResp {
	var (
		conv = make([]conver, len(kvs)+1)
	)
	for i := range kvs {
		conv[i].Speaker = AiSpeaker
		if (i % 2) == 0 {
			conv[i].Speaker = HumanSpeaker
		}
		conv[i].Speech = kvs[i]
	}
	return &ListResp{
		Id:     id,
		Title:  title,
		Conves: conv,
	}
}
