package main

import (
	"sync"
	"sync/atomic"
)

type TraceIdBatch struct {
	BatchPos     int
	ProcessCount int32
	TraceIdList  []byte
	Finish       bool
	rwLock       sync.RWMutex
	Begin		 bool
}

func NewTIB() *TraceIdBatch {
	return &TraceIdBatch{
		TraceIdList: make([]byte, 0, 512),
	}
}
func (tid *TraceIdBatch) setBegin() {
	tid.Begin = true
}
func (tid *TraceIdBatch) getBegin() bool {
	return tid.Begin
}
func (tid *TraceIdBatch) setFinish() {
	tid.Finish = true
}
func (tid *TraceIdBatch) getFinish() bool {
	return tid.Finish
}
func (tid *TraceIdBatch) setBatchPos(batchPos int) {
	tid.BatchPos = batchPos
}
func (tid *TraceIdBatch) getBatchPos() int {
	return tid.BatchPos
}
func (tid *TraceIdBatch) setProcessCount() int32 {
	return atomic.AddInt32(&tid.ProcessCount, 1)
}

func (tid *TraceIdBatch) getProcessCount() int32 {
	return atomic.LoadInt32(&tid.ProcessCount)
}
func (tid *TraceIdBatch) getTraceIdList() []byte {
	tid.rwLock.RLock()
	idList := tid.TraceIdList
	tid.rwLock.RUnlock()
	return idList
}

func (tid *TraceIdBatch) setTraceIdList(traceIdList []byte) {
	tid.rwLock.Lock()
	tid.TraceIdList = append(tid.TraceIdList, traceIdList...)
	tid.rwLock.Unlock()
}
