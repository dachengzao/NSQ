## 介绍
NSQ是一个基于Go语言的分布式实时消息平台，它基于MIT开源协议发布，由bitly公司开源出来的一款简单易用的消息中间件。

官方和第三方还为NSQ开发了众多客户端功能库，如官方提供的基于HTTP的nsqd、Go客户端go-nsq、Python客户端pynsq、基于Node.js的JavaScript客户端nsqjs、异步C客户端libnsq、Java客户端nsq-java以及基于各种语言的众多第三方客户端功能库。

NSQ是一个基于Go语言的分布式实时消息平台，可用于大规模系统中的实时消息服务，并且每天能够处理数亿级别的消息，其设计目标是为在分布式环境下运行的去中心化服务提供一个强大的基础架构。

NSQ非常容易配置和部署，且具有最大的灵活性，支持众多消息协议。

## Features

**1.Distributed** 

NSQ提供了分布式的，去中心化，且没有单点故障的拓扑结构，稳定的消息传输发布保障，能够具有高容错和HA（高可用）特性。

**2.Scalable易于扩展** 

NSQ支持水平扩展，没有中心化的brokers。内置的发现服务简化了在集群中增加节点。同时支持pub-sub和load-balanced 的消息分发。

**3.Ops Friendly** 

NSQ非常容易配置和部署，生来就绑定了一个管理界面。二进制包没有运行时依赖。官方有Docker image。

**4.Integrated高度集成** 

官方的 Go 和 Python库都有提供。而且为大多数语言提供了库。

## NSQ的三个优势：

- 1，性能。在多个著名网站生产环境中被采用，每天能够处理数亿级别的消息。参见官方提供的性能说明文档

- 2，易用性。非常易于部署（几乎没有依赖）和配置（所有参数都可以通过命令行进行配置）。

- 3，可扩展性。具有分布式且无单点故障的拓扑结构，支持水平扩展，在无中断情况下能够无缝地添加集群节点。还具有强大的集群管理界面，参见nsqadmin

## NSQ的三个进程：
- nsqd是一个接收、排队、然后转发消息到客户端的进程。
- nsqlookupd 管理拓扑信息并提供最终一致性的发现服务。
- nsqadmin用于实时查看集群的统计数据（并且执行各种各样的管理任务）。

### NSQ的四种重要组件构成：

- nsqd：一个负责接收、排队、转发消息到客户端的守护进程，它可以独立运行，不过通常它是由 nsqlookupd 实例所在集群配置的
- nsqlookupd：管理拓扑信息并提供最终一致性的发现服务的守护进程
- nsqadmin：一套Web用户界面，可实时查看集群的统计数据和执行各种各样的管理任务
- utilities：常见基础功能、数据流处理工具，如nsq_stat、nsq_tail、nsq_to_file、nsq_to_http、nsq_to_nsq、to_nsq

## NSQ数据流模型

NSQ中的数据流模型是由streams和consumers组成的tree。topic是一种独特的stream。channel是一个订阅了给定topic consumers 逻辑分组。
 
![](images/nsq-1.gif?raw=true)

**Topics 和 channels，是NSQ的核心成员。** 它们是如何使用go语言的特点来设计系统的最好示例。

**Topics**

Go的channels（为防止歧义，以下简称为“go-chan”）是表达队列的一种自然方式。

**一个NSQ的topic/channel，其核心就是一个存放消息指针的go-chan缓冲区。** 缓冲区的大小由  --mem-queue-size 配置参数确定。

读取数据后，向topic发布消息的行为包括：
- 实例化消息结构 (并分配消息体的字节数组)
- read-lock 并获得 Topic
- read-lock 并检查是否可以发布
- 发送到go-chan缓冲区

为了从一个topic和它的channels获得消息，topic不能按典型的方式用go-chan来接收，因为多个goroutines在一个go-chan上接收将会分发消息，而期望的结果是把每个消息复制到所有channel(goroutine)中。

此外，每个topic维护3个主要goroutine。

第一个，router，负责从传入的go-chan中读取新发布的消息，并存储到一个队列里（内存或硬盘）。

第二个，messagePump, 它负责复制和推送消息到如上所述的channel中。

第三个，DiskQueue IO，通过DiskQueue透明的把消息写入到磁盘上。将在后面讨论。

**Channels**

Channels稍微有点复杂，它的根本目的是向外暴露一个单输入单输出的go-chan（事实上从抽象的角度来说，消息可能存在内存里或硬盘上）；

![](images/nsq-2.png?raw=true)

另外，每一个channel维护2个时间优先级队列，用于延时和消息超时的处理（并有2个伴随goroutine来监视它们）。

并行化的改善是通过管理每个channel的数据结构来实现，而不是依靠go运行时的全局定时器。

注意：在内部，go运行时使用一个优先级队列和goroutine来管理定时器。它为整个time包（但不局限于）提供了支持。它通常不需要用户来管理时间优先级队列，但一定要记住，它是一个有锁的数据结构，有可能会影响 GOMAXPROCS>1 的性能。请参阅runtime/time.goc。

**Backend / DiskQueue**

NSQ的一个设计目标是绑定内存中的消息数目。它是通过DiskQueue(它拥有前面提到的的topic或channel的第三个goroutine)透明的把消息写入到磁盘上来实现的。

由于内存队列只是一个go-chan，没必要先把消息放到内存里，如果可能的话，退回到磁盘上：

``` go
for msg := range c.incomingMsgChan {
    select {
    case c.memoryMsgChan <- msg:
    default:
        err := WriteMessageToBackend(&msgBuf, msg, c.backend)
        if err != nil {
            // ... handle errors ...
        }
    }
}
```
利用go语言的select语句，只需要几行代码就可以实现这个功能：上面的default分支只有在memoryMsgChan 满的情况下才会执行。

NSQ也有临时channel的概念。临时channel会丢弃溢出的消息（而不是写入到磁盘），当没有客户订阅后它就会消失。

这是一个Go接口的完美用例。Topics和channels有一个的结构成员被声明为Backend接口，而不是一个具体的类型。一般的 topics和channels使用DiskQueue，而临时channel则使用了实现Backend接口的DummyBackendQueue。

## TCP 协议

NSQ的TCP协议是一个闪亮的会话典范，在这个会话中垃圾回收优化的理论发挥了极大的效用。协议的结构是一个有很长的前缀框架，这使得协议更直接，易于编码和解码。如下：

``` go
[x][x][x][x][x][x][x][x][x][x][x][x]...
|  (int32) ||  (int32) || (binary)
|  4-byte  ||  4-byte  || N-byte
------------------------------------...
    size      frame ID     data
```

- 因为框架的组成部分的确切类型和大小是提前知道的，所以我们可以规避了使用方便的编码二进制包的Read()和Write()封装（及它们外部接口的查找和会话）。反之，我们使用直接调用 binary.BigEndian方法。

- 为了消除socket输入输出的系统调用，客户端net.Conn被封装了bufio.Reader和bufio.Writer。这个Reader通过暴露ReadSlice()，复用了它自己的缓冲区。这样几乎消除了读完socket时的分配，这极大的降低了垃圾回收的压力。

这可能是因为与数据相关的大多数命令并没有逃逸（在边缘情况下这是假的，数据被强制复制）。

- 在更低层，MessageID 被定义为 [16]byte，这样可以将其作为 map 的 key（slice 无法用作 map 的 key)。然而，考虑到从 socket 读取的数据被保存为 []byte，胜于通过分配字符串类型的 key 来产生垃圾，并且为了避免从 slice 到 MessageID 的支撑数组产生复制操作，unsafe 包被用来将 slice 直接转换为 MessageID：

``` go
id := *(*nsq.MessageID)(unsafe.Pointer(&msgID))
```

注意: 这是个技巧。如果编译器对此已经做了优化，或者 Issue 3512 被打开可能会解决这个问题，那就不需要它了。issue 5376 也值得通读，它讲述了在无须分配和拷贝时，和 string 类型可被接收的地方，可以交换使用的“类常量”的 byte 类型。

- 类似的，Go 标准库仅仅在 string 上提供了数值转换方法。为了避免 string 的分配，nsqd 使用了惯用的十进制转换方法，用于对[]byte 直接操作。

这些看起来像是微优化，但 TCP 协议包含了一些最热的代码执行路径。总体来说，以每秒数万消息的速度来说，它们对分配和系统开销的数量有着显著的影响：

``` matlab
benchmark                    old ns/op    new ns/op    delta
BenchmarkProtocolV2Data           3575         1963  -45.09%
 
benchmark                    old ns/op    new ns/op    delta
BenchmarkProtocolV2Sub256        57964        14568  -74.87%
BenchmarkProtocolV2Sub512        58212        16193  -72.18%
BenchmarkProtocolV2Sub1k         58549        19490  -66.71%
BenchmarkProtocolV2Sub2k         63430        27840  -56.11%
 
benchmark                   old allocs   new allocs    delta
BenchmarkProtocolV2Sub256           56           39  -30.36%
BenchmarkProtocolV2Sub512           56           39  -30.36%
BenchmarkProtocolV2Sub1k            56           39  -30.36%
BenchmarkProtocolV2Sub2k            58           42  -27.59%
```


**HTTP**

NSQ的HTTP API是基于 Go's net/http 包实现的. 就是常见的HTTP应用,在大多数高级编程语言中都能直接使用而无需额外的三方包。

简洁就是它最有力的武器，Go的 HTTP tool-chest最强大的就是其调试功能.  net/http/pprof 包直接集成了HTTP server，可以方便的访问CPU, heap,goroutine, and OS 进程文档 .gotool就能直接实现上述操作:

``` go
$ go tool pprof http://127.0.0.1:4151/debug/pprof/profile
```

这对于调试和实时监控进程非常有用！

此外，/stats端端返回JSON或是美观的文本格式信息，这让管理员使用命令行实时监控非常容易:

``` gp
$ watch -n 0.5 'curl -s http://127.0.0.1:4151/stats | grep -v connected'
```

打印出的结果如下: NSQ

![](images/nsq-4.png?raw=true)

此外, Go还有很多监控指标measurable HTTP performance gains. 每次更新Go版本后都能看到性能方面的改进，真是让人振奋！

## 快速启动NSQ

```bash
brew install nsq
```

启动拓扑发现 

```bash
nsqlookupd
```

启动主服务、并注册 

```bash
nsqd --lookupd-tcp-address=127.0.0.1:4160
```

启动WEB UI管理程序 

```bash
nsqadmin --lookupd-http-address=127.0.0.1:4161
```

## 简单使用演示

可以用浏览器访问http://127.0.0.1:4171/观察数据

也可尝试下 watch -n 0.5 "curl -s http://127.0.0.1:4151/stats" 监控统计数据

发布一个消息 

```bash
curl -d 'hello world 1' 'http://127.0.0.1:4151/put?topic=test'
```

创建一个消费者 

```bash
nsq_to_file --topic=test --output-dir=/tmp --lookupd-http-address=127.0.0.1:4161
```

### Golang使用NSQ

go-nsq Golang客户端库（官方客户端开发库）。测试实例如下：


```go
package main

import (
    "fmt"
    "time"

    "github.com/nsqio/go-nsq"
)

// ConsumerHandler 消费者处理者
type ConsumerHandler struct{}

// HandleMessage 处理消息
func (*ConsumerHandler) HandleMessage(msg *nsq.Message) error {
    fmt.Println(string(msg.Body))
    return nil
}

// Producer 生产者
func Producer() {
    producer, err := nsq.NewProducer("127.0.0.1:4150", nsq.NewConfig())
    if err != nil {
        fmt.Println("NewProducer", err)
        panic(err)
    }

    i := 1
    for {
        if err := producer.Publish("test", []byte(fmt.Sprintf("Hello World %d", i))); err != nil {
            fmt.Println("Publish", err)
            panic(err)
        }

        time.Sleep(time.Second * 5)

        i++
    }
}

// ConsumerA 消费者
func ConsumerA() {
    consumer, err := nsq.NewConsumer("test", "test-channel-a", nsq.NewConfig())
    if err != nil {
        fmt.Println("NewConsumer", err)
        panic(err)
    }

    consumer.AddHandler(&ConsumerHandler{})

    if err := consumer.ConnectToNSQLookupd("127.0.0.1:4161"); err != nil {
        fmt.Println("ConnectToNSQLookupd", err)
        panic(err)
    }
}

// ConsumerB 消费者
func ConsumerB() {
    consumer, err := nsq.NewConsumer("test", "test-channel-b", nsq.NewConfig())
    if err != nil {
        fmt.Println("NewConsumer", err)
        panic(err)
    }

    consumer.AddHandler(&ConsumerHandler{})

    if err := consumer.ConnectToNSQLookupd("127.0.0.1:4161"); err != nil {
        fmt.Println("ConnectToNSQLookupd", err)
        panic(err)
    }
}

func main() {
    ConsumerA()
    ConsumerB()
    Producer()
}
```
命令执行顺序如下
```bash
nsqlookupd
nsqd --lookupd-tcp-address=127.0.0.1:4160 --broadcast-address=127.0.0.1
nsqadmin --lookupd-http-address=127.0.0.1:4161
```





















