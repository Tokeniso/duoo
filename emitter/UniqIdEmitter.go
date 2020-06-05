package emitter

import (
	"sync/atomic"
)

type UniqId struct {
	id int64
}

// 唯一ID生成器
type UniqIdEmitter struct {
	StartId int64
}

func (ue *UniqIdEmitter) MakePellets(ch chan interface{}, data interface{}) interface{} {
	defer func() {
		if recover() != nil {
			// TODO error log 忽略往关闭的chan中投放数据
		}
	}()
	if ue.StartId == 0 {
		ue.SetStartId(0)
	}
	uq := UniqId{
		id: atomic.AddInt64(&ue.StartId, 1),
	}
	ch <- uq
	return uq
}

// 可以启动速率控制前获取业务最新的id
func (ue *UniqIdEmitter) SetStartId(i int64) {
	if i == 0 {
		ue.StartId = 20060102150405
	} else {
		ue.StartId = i
	}
}
