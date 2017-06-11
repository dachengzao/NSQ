
# 分布式消息处理平台

## 这是**go-NSQ**的工程应用

NSQ是一个实时的分布式消息平台。它的设计目标是为在多台计算机上运行的松散服务提供一个现代化的基础设施骨架。

- NSQ 设计的目的是用来大规模地处理每天数以十亿计级别的消息。

- NSQ 具有分布式和去中心化拓扑结构，该结构具有无单点故障、故障容错、高可用性以及能够保证消息的可靠传递的特征。

- NSQ 非常容易配置和部署，且具有最大的灵活性，支持众多消息协议。另外，官方还提供了拆箱即用 Go 和 Python 库。


## NSQ的自身特色很明显，最主要的优势在如下三个方面：

- 1，性能。在多个著名网站生产环境中被采用，每天能够处理数亿级别的消息。参见官方提供的性能说明文档

- 2，易用性。非常易于部署（几乎没有依赖）和配置（所有参数都可以通过命令行进行配置）。

- 3，可扩展性。具有分布式且无单点故障的拓扑结构，支持水平扩展，在无中断情况下能够无缝地添加集群节点。还具有强大的集群管理界面，参见nsqadmin

## NSQ是由3个进程组成的：
- nsqd是一个接收、排队、然后转发消息到客户端的进程。
- nsqlookupd 管理拓扑信息并提供最终一致性的发现服务。
- nsqadmin用于实时查看集群的统计数据（并且执行各种各样的管理任务）。
- 
NSQ中的数据流模型是由streams和consumers组成的tree。topic是一种独特的stream。channel是一个订阅了给定topic consumers 逻辑分组。

![](images/nsq-1.gif?raw=true)

**Topics 和 Channels**

Topics 和 channels，是NSQ的核心成员，它们是如何使用go语言的特点来设计系统的最好示例。

Go的channels（为防止歧义，以下简称为“go-chan”）是表达队列的一种自然方式，因此一个NSQ的topic/channel，其核心就是一个存放消息指针的go-chan缓冲区。缓冲区的大小由  --mem-queue-size 配置参数确定。

读取数据后，向topic发布消息的行为包括：
- 实例化消息结构 (并分配消息体的字节数组)
- read-lock 并获得 Topic
- read-lock 并检查是否可以发布
- 发送到go-chan缓冲区

为了从一个topic和它的channels获得消息，topic不能按典型的方式用go-chan来接收，因为多个goroutines在一个go-chan上接收将会分发消息，而期望的结果是把每个消息复制到所有channel(goroutine)中。

此外，每个topic维护3个主要goroutine。第一个叫做 router，负责从传入的go-chan中读取新发布的消息，并存储到一个队列里（内存或硬盘）。

第二个，称为 messagePump, 它负责复制和推送消息到如上所述的channel中。

第三个负责 DiskQueue IO，将在后面讨论。

Channels稍微有点复杂，它的根本目的是向外暴露一个单输入单输出的go-chan（事实上从抽象的角度来说，消息可能存在内存里或硬盘上）；
![enter description here][2]

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

NSQ也有临时channel的概念。临时channel会丢弃溢出的消息（而不是写入到磁盘），当没有客户订阅后它就会消失。这是一个Go接口的完美用例。Topics和channels有一个的结构成员被声明为Backend接口，而不是一个具体的类型。一般的 topics和channels使用DiskQueue，而临时channel则使用了实现Backend接口的DummyBackendQueue。

## NSQ分布式实时消息平台用法

NSQ是一个基于Go语言的分布式实时消息平台，可用于大规模系统中的实时消息服务，并且每天能够处理数亿级别的消息，其设计目标是为在分布式环境下运行的去中心化服务提供一个强大的基础架构。

NSQ非常容易配置和部署，且具有最大的灵活性，支持众多消息协议。

### NSQ是由四个重要组件构成：

- nsqd：一个负责接收、排队、转发消息到客户端的守护进程，它可以独立运行，不过通常它是由 nsqlookupd 实例所在集群配置的
- nsqlookupd：管理拓扑信息并提供最终一致性的发现服务的守护进程
- nsqadmin：一套Web用户界面，可实时查看集群的统计数据和执行各种各样的管理任务
- utilities：常见基础功能、数据流处理工具，如nsq_stat、nsq_tail、nsq_to_file、nsq_to_http、nsq_to_nsq、to_nsq

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

## 快速启动NSQ

brew install nsq

启动拓扑发现 nsqlookupd

启动主服务、并注册 nsqd --lookupd-tcp-address=127.0.0.1:4160

启动WEB UI管理程序 nsqadmin --lookupd-http-address=127.0.0.1:4161

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