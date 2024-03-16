package merlin

import (
	"fmt"
	"time"

	"github.com/emirpasic/gods/queues/priorityqueue"
	"k8s.io/klog/v2"
)

var (
	// not use
	modelCost = map[string]int{
		"GPT 3":            1,
		"GPT 4":            30,
		"claude-instant-1": 2,
		"claude-2":         8,
		// t2i
		"Dreamshape v7": 10,
		"RPG v5":        10,
	}
	defaultImageModel = "Dreamshape v7"
	defaultChatModel  = "GPT 3"
)

type instance struct {
	idtoken     string // status
	accesstoken string // chat
	user        string
	password    string
	used        int
	limit       int
}

func (c *instance) DeepCopy() *instance {
	return &instance{
		accesstoken: c.accesstoken,
		idtoken:     c.idtoken,
		user:        c.user,
		password:    c.password,
		used:        c.used,
		limit:       c.limit,
	}
}

func instCompare(a, b interface{}) int {
	s1 := a.(*instance)
	s2 := b.(*instance)
	if s1.used > s2.used {
		return 1
	} else if s1.used == s2.used {
		return 0
	}
	return -1
}

type instCtrl struct {
	interval time.Duration

	ml *Merlin

	queue *priorityqueue.Queue
}

func NewInstControl(d time.Duration, ml *Merlin, user []*user) *instCtrl {
	queue := priorityqueue.NewWith(instCompare)
	for _, u := range user {
		in := &instance{
			user:     u.User,
			password: u.Password,
		}
		err := ml.refresh(in)
		if err != nil {
			klog.Error(err)
			continue
		}
		queue.Enqueue(in)
	}
	ic := &instCtrl{
		interval: d,
		ml:       ml,
		queue:    queue,
	}
	go ic.run()
	return ic
}

func (ic *instCtrl) Eequeue(m *instance) {
	ic.queue.Enqueue(m)
}

func (ic *instCtrl) Dequeue(m string) (*instance, error) {

	v, ok := ic.queue.Dequeue()
	if !ok {
		return nil, fmt.Errorf("not found instance")
	}
	inst := v.(*instance)
	// TODO: mostly 10
	if inst.limit-inst.used < 10 {
		ic.queue.Enqueue(v)
		return nil, fmt.Errorf("no avaliable instance to use")
	}
	return inst, nil
}

func (ic *instCtrl) run() {
	var interval = ic.interval
	for {
		iter := ic.queue.Iterator()
		for iter.Next() {
			v, ok := iter.Value().(*instance)
			if !ok {
				continue
			}
			if v.accesstoken != "" && ic.ml.status(v) == nil {
				klog.Infof("user(%s/%s) limit %d used %d.", v.user, v.password, v.limit, v.used)
				continue
			}
			err := ic.ml.refresh(v)
			if err != nil {
				klog.Errorf("merlin user(%s/%s) login failed: %v", v.user, v.password, err)
			} else {
				klog.Infof("user(%s/%s) limit %d used %d.", v.user, v.password, v.limit, v.used)
			}
		}

		<-time.NewTicker(interval).C
	}
}
