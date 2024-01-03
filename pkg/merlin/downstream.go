package merlin

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/yylt/chatmux/pkg"
	"github.com/yylt/chatmux/pkg/util"
	"k8s.io/klog/v2"
)

var (
	messagePrefix = []byte("message")
	dataPrefix    = []byte("data:")
)

type respReadClose struct {
}

func NewRespRead() *respReadClose {
	rr := &respReadClose{}
	return rr
}

func (rr *respReadClose) Reader(raw io.ReadCloser) <-chan *pkg.BackResp {

	var (
		schan = make(chan *pkg.BackResp, 4)
	)

	go func(ra io.ReadCloser, sch chan *pkg.BackResp) {
		var (
			respData = &EventResp{}
			err      error
		)
		evch, errch := util.StartLoop(ra)

		defer func() {
			ra.Close()
			close(errch)
			close(evch)
			close(sch)
		}()

		for {
			select {
			case v, ok := <-evch:
				klog.Infof("get event data: %v", v)
				if !ok {
					return
				}

				if !bytes.Equal(v.Event, messagePrefix) {
					continue
				}
				err = json.Unmarshal(v.Data, &respData)
				if err != nil {
					klog.Errorf("parse event stream data failed: %v", err)
					continue
				}

				evdata := respData.Data

				klog.Infof("read data: %v", evdata)
				switch evdata.Type {
				case string(chunk):
					sch <- &pkg.BackResp{
						Content: evdata.Content,
					}
				default:
				}
			case err := <-errch:
				sch <- &pkg.BackResp{
					Err: err,
				}
				return
			}
		}
	}(raw, schan)

	return schan
}
