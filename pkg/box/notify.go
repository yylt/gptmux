package box

import (
	"fmt"

	"k8s.io/klog/v2"
)

type NotifyConf struct {
	// email
	Email *emailConf `yaml:"email,omitempty"`

	// weixin
	FtangToken string `yaml:"tang,omitempty"`

	// gnotify
	Gnt *gtifyconf `yaml:"gnotify,omitempty"`
}

type Notify struct {
	pushs []Notifyer

	pulls []Receiver
}

func New(conf *NotifyConf) Box {
	nt := &Notify{}

	em := newEmail(conf.Email)
	if em != nil {
		nt.add(em)
	}
	t := newTang(conf.FtangToken)
	if t != nil {
		nt.add(t)
	}
	gnt := newGnotify(conf.Gnt)
	if gnt != nil {
		nt.add(gnt)
	}
	return nt
}
func (nf *Notify) Name() string {
	return "manager"
}
func (nf *Notify) add(a any) {

	n1, ok := a.(Notifyer)
	if ok {
		klog.Infof("add notifyer: %s", n1.Name())
		nf.pushs = append(nf.pushs, n1)
	}
	n2, ok := a.(Receiver)
	if ok {
		klog.Infof("add receiver: %s", n1.Name())
		nf.pulls = append(nf.pulls, n2)
	}
}

func (nf *Notify) Push(m *Message) error {
	if len(nf.pushs) == 0 {
		return fmt.Errorf("no avaliable notifyer")
	}
	var (
		rerr error
	)
	for _, ps := range nf.pushs {
		err := ps.Push(m)
		if err != nil {
			klog.Errorf("notify failed: %v", err)
			rerr = err
		} else {
			rerr = nil
		}
	}
	return rerr
}

func (nf *Notify) Receive(fn func(*Message) bool) error {
	if len(nf.pulls) == 0 {
		return fmt.Errorf("no avaliable receive")
	}
	var (
		rerr error
	)
	for _, pl := range nf.pulls {
		err := pl.Receive(fn)
		if err != nil {
			klog.Errorf("receive failed: %v", err)
			rerr = err
		} else {
			rerr = nil
		}
	}
	return rerr
}
