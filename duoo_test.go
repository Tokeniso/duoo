package duoo

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Tokeniso/duoo/emitter"
	"github.com/Tokeniso/duoo/tool"
)

// 布尔速率许可分配测试
func TestRateStartWithDebug(t *testing.T) {
	bc := RateStartWithDebug(50000000, 1)
	wg := sync.WaitGroup{}
	for i := 0; i < 30000; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for j := 0; j < 5000000; j++ {
				_, err := bc.GetPermission()
				if err != nil {
					//t.Logf("err: %s", err)
					break
				}
			}
		}()
	}
	time.AfterFunc(9011*time.Millisecond, func() {
		bc.Stop()
	})
	wg.Wait()

	for key, val := range bc.Count {
		t.Logf("循环批次：%d -- wait：%d -- retry：%d -- done：%d -- make：%d\n",
			key, val.Wait, val.Retry, val.Done, val.Make)
	}
}

// 布尔速率许可分配测试
func TestRateStart(t *testing.T) {
	bc := RateStart(300000, 1)

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	for i := 0; i < 30000; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for j := 0; j < 50000; j++ {
				_, err := bc.GetPermission()
				if err != nil {
					//t.Logf("err: %s", err)
					break
				}
				tag := bc.GetTag()
				mu.Lock()
				_, ok := bc.Count[tag]
				if !ok {
					bc.Count[tag] = &tool.Statistics{0, 0, 0, 0}
				}
				bc.Count[tag].Done++
				mu.Unlock()
			}
		}()
	}
	time.AfterFunc(4011*time.Millisecond, func() {
		bc.Stop()
	})
	wg.Wait()

	for key, val := range bc.Count {
		t.Logf("循环批次：%d -- wait：%d -- retry：%d -- done：%d -- make：%d\n",
			key, val.Wait, val.Retry, val.Done, val.Make)
	}
}


// 唯一ID速率许可分配测试
func TestStartEmitterWithDebug(t *testing.T) {
	ei := emitter.UniqIdEmitter{}

	bc := StartEmitterWithDebug(50000000, 1, &ei)
	wg := sync.WaitGroup{}
	for i := 0; i < 30000; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for j := 0; j < 5000000; j++ {
				a, err := bc.GetPermission()

				fmt.Println(a)
				if err != nil {
					//t.Logf("err: %s", err)
					break
				}
			}
		}()
	}
	time.AfterFunc(9011*time.Millisecond, func() {
		bc.Stop()
	})
	wg.Wait()

	for key, val := range bc.Count {
		t.Logf("循环批次：%d -- wait：%d -- retry：%d -- done：%d -- make：%d\n",
			key, val.Wait, val.Retry, val.Done, val.Make)
	}
}