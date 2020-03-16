package duoo

import (
	"sync"
	"testing"
	"time"
)

// 速率许可分配测试
func TestNewRateWithDebug(t *testing.T) {
	bc := RateStartWithDebug(300000, 1)

	wg := sync.WaitGroup{}
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
			}
		}()
	}
	time.AfterFunc(4011*time.Millisecond, func() {
		bc.Stop()
	})
	wg.Wait()

	for key, val := range bc.Count {
		t.Logf("循环批次：%d -- wait：%d -- retry：%d -- done：%d\n", key, val.Wait, val.Retry, val.Done)
	}
}

// 速率许可分配测试
func TestNewRate(t *testing.T) {
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
					bc.Count[tag] = &Statistics{0, 0, 0}
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
		t.Logf("循环批次：%d -- done：%d\n", key, val.Done)
	}
}
