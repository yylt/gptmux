package merlin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/yylt/chatmux/pkg"
	"k8s.io/klog/v2"
)

var (
	message = []byte("message")

	UnauthErr = errors.New("unauth")
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

	go func(raw io.ReadCloser, sch chan *pkg.BackResp) {
		var (
			respData = &merelinResp{}
			err      error
		)
		evch, errch := pkg.StartLoop(raw)

		defer func() {
			raw.Close()
			close(errch)
			close(evch)
			close(sch)
		}()

		for {
			select {
			case v, ok := <-evch:
				if !ok {
					return
				}
				if !bytes.Equal(v.Event, message) {
					continue
				}
				err = json.Unmarshal(v.Data, &respData)
				if err != nil {
					klog.Errorf("parse event stream data failed: %v", err)
					continue
				}

				evdata, ok := respData.Data.(*eventData)
				if !ok {
					klog.Errorf("it is not event data")
					return
				}
				klog.V(2).Infof("read data: %v", evdata)
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
