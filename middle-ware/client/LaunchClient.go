package main

import (
	"context"
	"encoding/json"
	"log"
	"middle-ware/util"
	"net"
	"net/http"
	"strconv"
)

type launchClient struct {
	TraceDataPort string
	SetWTIConn    *util.TcpClient
	GetWTConn     *util.TcpClient
}

func NewLaunchClient() *launchClient {
	return &launchClient{}
}
func (lc *launchClient) LaunchClient(startPort string) {
	m := http.NewServeMux()
	s := http.Server{Addr: ":" + startPort, Handler: m}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		//打开连接:
		for {
			conn1, err := net.Dial("tcp", "localhost:8003")
			if err == nil {
				lc.SetWTIConn = util.NewTcpClient(conn1)
				break
			}
		}

		for {
			conn2, err := net.Dial("tcp", "localhost:8004")
			if err == nil {
				lc.GetWTConn = util.NewTcpClientSize(conn2, 262144)
				break
			}
		}
		go lc.GetWrongTrace()
		log.Printf("ready success\n")
		w.Write([]byte("suc"))
		// Cancel the context on request
	})

	m.HandleFunc("/setParameter", func(w http.ResponseWriter, r *http.Request) {
		lc.TraceDataPort = r.FormValue("port")
		w.Write([]byte("suc"))
		log.Printf("setParameter success\n")
		cancel()
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	select {
	case <-ctx.Done():
		s.Shutdown(ctx)
	}
	log.Printf("Ready Finished ")
}

func (lc *launchClient) GetWrongTrace() {
	for {
		buf, _ := lc.GetWTConn.Read()
		length := len(buf)
		if length > 0 {
			wrongTraceMap := make(map[string]*[]string, 64) //256kB
			dataBytes := make([]byte, 0, 16)
			traceIdList := make([]string, 0, 64)
			var batchPos int
			for i := 0; i < length; i++ {
				if buf[i] == '|' {
					traceIdList = append(traceIdList, util.Bytes2str(dataBytes))
					dataBytes = make([]byte, 0, 16)
				} else if buf[i] == ',' {
					batchPos, _ = strconv.Atoi(util.Bytes2str(buf[i+1 : length]))
					break
				} else {
					dataBytes = append(dataBytes, buf[i])
				}
			}
			log.Printf("receive BatPos:%d\n", batchPos)
			pos := batchPos % BatchCount
			previous := pos - 1
			if previous == -1 {
				previous = BatchCount - 1
			}
			next := pos + 1
			if next == BatchCount {
				next = 0
			}
			for _, traceId := range traceIdList {
				if _, ok := wrongTraceMap[traceId]; ok {
					continue
				}
				spanListPtr := new([]string)
				*spanListPtr = make([]string, 0, 64)
				wrongTraceMap[traceId] = spanListPtr
				traceMap := BatchTraceList[previous]
				if spanList, ok := traceMap[traceId]; ok {
					*spanListPtr = append(*spanListPtr, *spanList...)
				}

				traceMap = BatchTraceList[pos]
				if spanList, ok := traceMap[traceId]; ok {
					*spanListPtr = append(*spanListPtr, *spanList...)
				}
				traceMap = BatchTraceList[next]
				if spanList, ok := traceMap[traceId]; ok {
					*spanListPtr = append(*spanListPtr, *spanList...)
				}
				if len(*spanListPtr) <= 0 {
					delete(wrongTraceMap, traceId)
				}
			}
			msg, _ := json.Marshal(wrongTraceMap)
			lc.GetWTConn.Write(msg)
			BatchTraceList[previous] = make(map[string]*[]string, 8192)
		}
	}
}

func (lc *launchClient) UpdateWrongTraceIdThread(badTraceIdList []byte, batchPos int) {
	badTraceIdList = append(badTraceIdList, util.Str2bytes(","+strconv.Itoa(batchPos))...)
	log.Printf("send batchPos:%d   badTraceIdList: %+v\n", batchPos, util.Bytes2str(badTraceIdList))
	lc.SetWTIConn.Write(badTraceIdList)
}

func (lc *launchClient) handlerLine(line []byte) {
	if len(line) > 1 {
		count++
		place := 0
		for index, sign := range line {
			if sign == '|' {
				place = index
				break
			}
		}
		traceId := line[0:place]

		if tmp, ok := BatchTraceList[pos][util.Bytes2str(traceId)]; ok {
			*tmp = append(*tmp, util.Bytes2str(line))
		} else {
			spanList := new([]string)
			*spanList = make([]string, 0, 32)
			BatchTraceList[pos][util.Bytes2str(traceId)] = spanList
			*spanList = append(*spanList, util.Bytes2str(line))
		}

		if isContain(line) {
			badCount++
			BadTraceIdList = append(BadTraceIdList, traceId...)
			BadTraceIdList = append(BadTraceIdList, '|')
		}
	}
	if count%util.BatchSize == 0 {
		pos++
		go func(traceList []byte, batchPos int) {
			lc.UpdateWrongTraceIdThread(BadTraceIdList, count/util.BatchSize-1)
		}(BadTraceIdList, count)
		BadTraceIdList = make([]byte, 0, 512)
	}
}

func isContain(line []byte) bool {
	for i := len(line) - 1; i > 0; i-- {
		if line[i] == '1' && line[i-1] == '=' && line[i-2] == 'r' && line[i-3] == 'o' &&
			line[i-4] == 'r' && line[i-5] == 'r' && line[i-6] == 'e' {
			return true
		}
		if line[i] == 'e' && line[i-1] == 'd' && line[i-2] == 'o' && line[i-3] == 'c' &&
			line[i-4] == '_' && line[i-5] == 's' && line[i-6] == 'u' && line[i-7] == 't' &&
			line[i-8] == 'a' && line[i-9] == 't' && line[i-10] == 's' && line[i-11] == '.' &&
			line[i-12] == 'p' && line[i-13] == 't' && line[i-14] == 't' && line[i-15] == 'h' {
			if line[i+1] == '=' && (line[i+2] != '2' || line[i+3] != '0' || line[i+4] != '0') {
				return true
			}
		}
		if line[i] == '|' {
			return false
		}
	}
	return false
}
