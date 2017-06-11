package nsqlookupd

import (
	"log"
	"net"
	"os"
	"sync"

	"github.com/nsqio/nsq/internal/http_api"
	"github.com/nsqio/nsq/internal/lg"
	"github.com/nsqio/nsq/internal/protocol"
	"github.com/nsqio/nsq/internal/util"
	"github.com/nsqio/nsq/internal/version"
)

// nsqlookupd是守护进程负责管理拓扑信息。客户端通过查询nsqlookupd来发现指定话题（topic）的生产者，
// 并且nsqd节点广播话题（topic）和通道（channel）信息。
// nsqlookupd有两个接口：TCP接口，nsqd用它来广播。HTTP接口，客户端用它来发现和管理。

type NSQLookupd struct {
	sync.RWMutex          //读写锁
	opts         *Options // nsqlookupd 配置信息 定义文件路径为nsq/nsqlookupd/options.go
	tcpListener  net.Listener
	httpListener net.Listener
	waitGroup    util.WaitGroupWrapper // WaitGroup 典型应用 用于开启两个goroutine，一个监听HTTP 一个监听TCP
	DB           *RegistrationDB       // product 注册数据库
}

// 初始化NSQLookupd实例
func New(opts *Options) *NSQLookupd {
	if opts.Logger == nil {
		opts.Logger = log.New(os.Stderr, opts.LogPrefix, log.Ldate|log.Ltime|log.Lmicroseconds)
	}
	n := &NSQLookupd{
		opts: opts,
		DB:   NewRegistrationDB(), // 初始化DB实例
	}

	var err error
	opts.logLevel, err = lg.ParseLogLevel(opts.LogLevel, opts.Verbose)
	if err != nil {
		n.logf(LOG_FATAL, "%s", err)
		os.Exit(1)
	}

	n.logf(LOG_INFO, version.String("nsqlookupd"))
	return n
}

func (l *NSQLookupd) Main() {
	// 初始化Context实例将NSQLookupd指针放入Context实例中 Context结构
	// 请参考文件nsq/nsqlookupd/context.go Context用于nsqlookupd中的tcpServer 和 httpServer中
	ctx := &Context{l}

	tcpListener, err := net.Listen("tcp", l.opts.TCPAddress) //开启TCP监听
	if err != nil {
		l.logf(LOG_FATAL, "listen (%s) failed - %s", l.opts.TCPAddress, err)
		os.Exit(1)
	}
	l.Lock()
	l.tcpListener = tcpListener
	l.Unlock()

	// 创建一个tcpServer tcpServer 实现了nsq/internal/protocol包中的TCPHandler接口
	tcpServer := &tcpServer{ctx: ctx}
	l.waitGroup.Wrap(func() {
		// protocol.TCPServer方法的过程就是tcpListener accept tcp的连接
		// 然后通过tcpServer中的Handle分析报文，然后处理相关的协议
		protocol.TCPServer(tcpListener, tcpServer, l.logf)
	}) // 把tcpServer加入到waitGroup

	httpListener, err := net.Listen("tcp", l.opts.HTTPAddress) // 开启HTTP监听
	if err != nil {
		l.logf(LOG_FATAL, "listen (%s) failed - %s", l.opts.HTTPAddress, err)
		os.Exit(1)
	}
	l.Lock()
	l.httpListener = httpListener
	l.Unlock()
	httpServer := newHTTPServer(ctx) // 创建一个httpServer
	l.waitGroup.Wrap(func() {
		http_api.Serve(httpListener, httpServer, "HTTP", l.logf)
	}) // 把httpServer加入到waitGroup
}

func (l *NSQLookupd) RealTCPAddr() *net.TCPAddr {
	l.RLock()
	defer l.RUnlock()
	return l.tcpListener.Addr().(*net.TCPAddr)
}

func (l *NSQLookupd) RealHTTPAddr() *net.TCPAddr {
	l.RLock()
	defer l.RUnlock()
	return l.httpListener.Addr().(*net.TCPAddr)
}

// NSQLookupd退出
func (l *NSQLookupd) Exit() {
	if l.tcpListener != nil {
		l.tcpListener.Close()
	}

	if l.httpListener != nil {
		l.httpListener.Close()
	}
	l.waitGroup.Wait()
}
