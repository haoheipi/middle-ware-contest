package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"middle-ware/util"
	"middle-ware/util/concurrent_map"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
)

var BatchCount int32
var FinishProcessCount int32
var CurrentBatch int32
var TIBList []*TraceIdBatch
var TraceCheckSumMap concurrent_map.ConcurrentMap
var logger *log.Logger
var LastBatch int32

func main() {
	//runtime.GOMAXPROCS(2)
	FinishProcessCount = 0
	CurrentBatch = 0
	LastBatch = math.MaxInt32
	BatchCount = 1024
	TIBList = make([]*TraceIdBatch, 0, BatchCount)
	for i := int32(0); i < BatchCount; i++ {
		TIBList = append(TIBList, NewTIB())
	}
	TraceCheckSumMap = concurrent_map.New()
	//启动端口
	var wg sync.WaitGroup

	startPort := os.Args[1]
	log.Println("Starting the server ...")
	launchService := NewLaunchService()
	//监听客户端tcp连接
	wg.Add(1)
	go func() {
		defer wg.Done()
		launchService.Listen()
	}()

	//监听http连接
	wg.Add(1)
	go func() {
		defer wg.Done()
		launchService.LaunchService(startPort)
	}()
	wg.Wait()

	for {
		traceIdBatch := GetFinishedBatch()
		if traceIdBatch == nil {
			if isFinished() {
				log.Printf("处理结束\n")
				break
			}
			continue
		}
		log.Printf("handTraceBatch=%d\n", traceIdBatch.BatchPos)
		launchService.getWrongTrace(traceIdBatch)
		//go trace
	}

	bytesData, _ := json.Marshal(TraceCheckSumMap)
	req, err := http.PostForm("http://localhost:"+launchService.TraceDataPort+"/api/finished", url.Values{
		"result": []string{string(bytesData)},
	})
	if err != nil {
		log.Println("==1",err.Error())
		return
	}
	defer req.Body.Close()
	_, err = ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println("==2",err.Error())
	}
	wg.Add(1)
	wg.Wait()

}

func GetFinishedBatch() *TraceIdBatch {
	current := atomic.LoadInt32(&CurrentBatch)
	next := current + 1
	if next >= BatchCount {
		next = 0
	}
	currentBatch := TIBList[current]
	nextBatch := TIBList[next]

	if (nextBatch.getProcessCount() >= util.ProcessCount && currentBatch.getProcessCount() >= util.ProcessCount && !currentBatch.getBegin()) ||
		(atomic.LoadInt32(&FinishProcessCount) >= util.ProcessCount && (!currentBatch.getFinish() && currentBatch.getProcessCount() >= util.ProcessCount) && !currentBatch.getBegin()) {
		log.Printf("currentBatch:%d  isBegin:%+v isFinish:%+v ProcessCount:%+v", current, currentBatch.getBegin(), currentBatch.getFinish(), currentBatch.getProcessCount())
		log.Printf("nextBatch:   %d  isBegin:%+v isFinish:%+v ProcessCount:%+v", next, nextBatch.getBegin(), nextBatch.getFinish(), nextBatch.getProcessCount())
		nTIB := NewTIB() // 清空操作可以优化？
		TIBList[current] = nTIB
		currentBatch.setBegin()
		return currentBatch
	}
	return nil
}

func isFinished() bool {
	if atomic.LoadInt32(&CurrentBatch) > atomic.LoadInt32(&LastBatch) {
		log.Printf("CurrentBatch=%d LastBatch=%d\n", atomic.LoadInt32(&CurrentBatch), atomic.LoadInt32(&LastBatch))
		return true
	}
	return false
}
