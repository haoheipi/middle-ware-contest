package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"middle-ware/util"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

type launchService struct {
	listenerSet   net.Listener
	listenerGet   net.Listener
	TraceDataPort string
	sendConn      []*util.TcpClient
}

func NewLaunchService() *launchService {
	listenerSet, _ := net.Listen("tcp", "localhost:8003")
	listenerGet, _ := net.Listen("tcp", "localhost:8004")
	return &launchService{
		listenerSet: listenerSet,
		listenerGet: listenerGet,
		sendConn:    make([]*util.TcpClient, 0, 2),
	}
}

func (ls *launchService) Listen() {
	// 监听并接受来自客户端的连接
	for i := 0; i < 2; i++ {
		conn1, _ := ls.listenerSet.Accept()
		go handleSetTraceId(util.NewTcpClientSize(conn1, 131072*4))
		conn2, _ := ls.listenerGet.Accept()
		ls.sendConn = append(ls.sendConn, util.NewTcpClientSize(conn2, 131072*1024))
		//go handleGetWrongTrace(conn[i])
	}
}

func (ls *launchService) LaunchService(startPort string) {
	m := http.NewServeMux()
	s := http.Server{Addr: ":" + startPort, Handler: m}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("suc"))
		// Cancel the context on request
		log.Printf("ready suc\n")
	})

	m.HandleFunc("/setParameter", func(w http.ResponseWriter, r *http.Request) {
		ls.TraceDataPort = r.FormValue("port")
		w.Write([]byte("suc"))
		// Cancel the context on request
		log.Printf("setParameter suc\n")
		cancel()
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		s.Shutdown(ctx)
	}
	log.Printf("Ready Finished ")
}
func (ls *launchService) getWrongTrace(traceIdBatch *TraceIdBatch) {
	traceIdList := append(traceIdBatch.TraceIdList, util.Str2bytes(","+strconv.Itoa(traceIdBatch.getBatchPos()))...)
	var wg sync.WaitGroup

	var m1 map[string][]string
	var m2 map[string][]string
	wg.Add(1)
	go func() {
		defer wg.Done()
		ls.sendConn[0].Write(traceIdList)
		traceList, _ := ls.sendConn[0].Read()
		json.Unmarshal(traceList, &m1)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		ls.sendConn[1].Write(traceIdList)
		traceList, _ := ls.sendConn[1].Read()
		json.Unmarshal(traceList, &m2)
	}()
	wg.Wait()

	go sortAndMd5(m1, m2, traceIdBatch)
}

func handleSetTraceId(in *util.TcpClient) {

	defer in.Close()
	var buf []byte
	var err error
	for {
		buf, err = in.Read()
		if err != nil {
			log.Printf("====Error reading %+v：%+v\n", err.Error(), buf)
			return //终止程序
		}
		log.Printf("receive badTraceId:%+v\n", util.Bytes2str(buf))
		length := len(buf)
		// 123456|345678|123451|,batchPos

		var batchPos int
		if buf[0] == '!' {
			//fmt.Println("发送完成")
			atomic.AddInt32(&FinishProcessCount, 1)
			lastBatch, _ := strconv.Atoi(util.Bytes2str(buf[1:]))
			atomic.SwapInt32(&LastBatch, int32(lastBatch))
			log.Printf("FinishProcessCount:%d", atomic.LoadInt32(&FinishProcessCount))
			log.Printf("lastBatch:%d", atomic.LoadInt32(&LastBatch))
			return
		}
		i := 0
		for ; i < length; i++ {
			if buf[i] == ',' {
				batchPos, err = strconv.Atoi(util.Bytes2str(buf[i+1 : length]))
				break
			}
		}
		pos := int32(batchPos) % BatchCount
		tid := TIBList[pos]
		if i >= 0 {
			tid.setBatchPos(batchPos)
			tid.setProcessCount()
			tid.setTraceIdList(buf[0:i])
		}
	}

}

func sortAndMd5(map1 map[string][]string, map2 map[string][]string, batch *TraceIdBatch) {
	m := make(map[string][]sortItem, 64)
	for traceId, spanList := range map1 {
		for _, span := range spanList {
			var createTime string
			spanByte := util.Str2bytes(span)
			length := len(spanByte)
			for i := 0; i < length; i++ {
				if spanByte[i] == '|' {
					j := i + 1
					for ; j < length; j++ {
						if spanByte[j] == '|' {
							createTime = util.Bytes2str(spanByte[i+1 : j])
							break
						}
					}
					break
				}
			}
			m[traceId] = append(m[traceId], sortItem{
				createTime: createTime,
				data:       span,
			})
		}
	}
	for traceId, spanList := range map2 {
		for _, span := range spanList {
			var createTime string
			spanByte := util.Str2bytes(span)
			length := len(spanByte)

			for i := 0; i < length; i++ {
				if spanByte[i] == '|' {
					j := i + 1
					for ; j < length; j++ {
						if spanByte[j] == '|' {
							createTime = util.Bytes2str(spanByte[i+1 : j])
						}
					}
					break
				}
			}
			m[traceId] = append(m[traceId], sortItem{
				createTime: createTime,
				data:       span,
			})
		}
	}
	for k, v := range m {
		data := make([]byte, 0, 32768)
		sort.Slice(v, func(i, j int) bool {
			return v[i].createTime < v[j].createTime
		})

		for _, span := range v {
			data = append(data, util.Str2bytes(span.data)...)
		}

		sum := md5.Sum(data)
		md5 := hex.EncodeToString(sum[:])
		TraceCheckSumMap.Set(k, md5)
	}
	batch.setFinish()
	atomic.AddInt32(&CurrentBatch, 1)
	log.Printf("after hand, currentBatch:%d", atomic.LoadInt32(&CurrentBatch))
}

type sortItem struct {
	createTime string
	data       string
}
