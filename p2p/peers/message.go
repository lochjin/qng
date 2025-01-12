package peers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/p2p/encoder"
	"github.com/libp2p/go-libp2p/core/network"
	"io"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

const MaxMessageSize = 10 * 1024 * 1024
const MsgCodeSize = 8
const PacketSize = 8

// HandleTimeout is the maximum time for complete handler.
const HandleTimeout = 20 * time.Second

var ErrConnClosed = errors.New("ConnMsg: read or write on closed message")

type Msg struct {
	ID      uint64
	Code    uint64
	Payload []byte

	ReceivedAt time.Time
	Reply      chan interface{}
}

func (msg *Msg) Decode(val interface{}) error {

	return nil
}

func (msg *Msg) Size() int {
	return len(msg.Payload) + MsgCodeSize*2
}

func (msg *Msg) String() string {
	if msg.ReceivedAt.Unix() > 0 {
		return fmt.Sprintf("id:%d code:%d size:%d time:%s", msg.ID, msg.Code, msg.Size(), msg.ReceivedAt.String())
	} else {
		return fmt.Sprintf("id:%d code:%d size:%d", msg.ID, msg.Code, msg.Size())
	}
}

func (msg *Msg) Time() time.Time {
	return msg.ReceivedAt
}

type MsgHander func(id uint64, msg interface{}, pe *Peer) error

var MsgHanders = map[uint64]MsgHander{}
var MsgDataTypes = map[uint64]interface{}{}

func RegisterHandler(code uint64, base interface{}, handler MsgHander) {
	MsgHanders[code] = handler
	MsgDataTypes[code] = base
}

func RegisterDataType(code uint64, base interface{}) {
	MsgDataTypes[code] = base
}

type ConnMsgRW struct {
	w       chan *Msg
	closing chan struct{}
	closed  *atomic.Bool
	rw      *bufio.ReadWriter
	en      encoder.NetworkEncoding
	wg      sync.WaitGroup
	pending sync.Map
}

func (p *ConnMsgRW) Send(msgcode uint64, data interface{}, respondID uint64) (interface{}, error) {
	if p.closed.Load() {
		return nil, ErrConnClosed
	}
	_, ok := MsgDataTypes[msgcode]
	if !ok {
		return nil, fmt.Errorf("Not support message:code=%d", msgcode)
	}
	id := respondID
	if id == 0 {
		id = rand.Uint64()
	}

	var buff bytes.Buffer
	_, err := p.Encoder().EncodeWithMaxLength(&buff, data)
	if err != nil {
		return nil, err
	}
	payload := buff.Bytes()
	if len(payload) <= 0 {
		return nil, fmt.Errorf("Empty data:%d", msgcode)
	}
	msg := &Msg{ID: id, Code: msgcode, Payload: payload}
	if respondID == 0 {
		msg.Reply = make(chan interface{})
	}
	log.Debug("Send message", "msg", msg.String())
	select {
	case p.w <- msg:
	case <-p.closing:
		return nil, ErrConnClosed
	}
	if msg.Reply != nil {
		select {
		case ret := <-msg.Reply:
			return ret, nil
		case <-p.closing:
			return nil, ErrConnClosed
		}
	}
	return nil, nil
}

func (p *ConnMsgRW) Close() error {
	if p.closed.Swap(true) {
		// someone else is already closing
		return nil
	}
	close(p.closing)
	p.wg.Wait()
	return nil
}

func (p *ConnMsgRW) Run(pe *Peer) error {
	if p.closed.Load() {
		return ErrConnClosed
	}
	var (
		readErr = make(chan error)
	)
	p.wg.Add(1)
	go p.readLoop(pe, readErr)

	log.Info("Enter ConnMsgRW", "peer", pe.GetID())
	defer func() {
		p.Close()
		log.Info("End ConnMsgRW", "peer", pe.GetID())
	}()

loop:
	for {
		select {
		case msg := <-p.w:
			dataSize := msg.Size()
			if dataSize > MaxMessageSize {
				if msg.Reply != nil {
					msg.Reply <- nil
				}
				return fmt.Errorf("Too large message size: %d > %d", dataSize, MaxMessageSize)
			}
			bs := make([]byte, PacketSize)
			binary.BigEndian.PutUint64(bs, uint64(dataSize))
			size, err := p.rw.Write(bs)
			if err != nil {
				return err
			}
			if size != PacketSize {
				return fmt.Errorf("Write size error:%d", size)
			}
			bs = make([]byte, MsgCodeSize)
			binary.BigEndian.PutUint64(bs, msg.ID)

			size, err = p.rw.Write(bs)
			if err != nil {
				return err
			}
			if size != MsgCodeSize {
				return fmt.Errorf("write size error:%d", size)
			}
			bs = make([]byte, MsgCodeSize)
			binary.BigEndian.PutUint64(bs, msg.Code)

			size, err = p.rw.Write(bs)
			if err != nil {
				return err
			}
			if size != MsgCodeSize {
				return fmt.Errorf("write size error:%d", size)
			}
			size, err = p.rw.Write(msg.Payload)
			if err != nil {
				return err
			}
			err = p.rw.Flush()
			if err != nil {
				return err
			}
			pe.IncreaseBytesSent(msg.Size() + PacketSize)
			if msg.Reply != nil {
				p.pending.Store(msg.ID, msg.Reply)
			}

		case err := <-readErr:
			return err

		case <-p.closing:
			break loop
		}
	}
	return nil
}

func (p *ConnMsgRW) Encoder() encoder.NetworkEncoding {
	return p.en
}

func (p *ConnMsgRW) readLoop(pe *Peer, errc chan<- error) {
	defer p.wg.Done()
	returnFun := func(err error) {
		if err != nil {
			select {
			case <-p.closing:
				return
			case errc <- err:
			}
		}
	}
	for {
		msg, err := p.readMsg(pe)
		if err != nil {
			returnFun(err)
			return
		}
		if msg == nil {
			returnFun(fmt.Errorf("No read msg"))
			return
		}
		msg.ReceivedAt = time.Now()
		//
		base, ok := MsgDataTypes[msg.Code]
		if !ok {
			returnFun(fmt.Errorf("Unknown message type code: msg=%s, %s", msg.String(), pe.GetID().String()))
			return
		}
		t := reflect.TypeOf(base)
		var ty reflect.Type
		if t.Kind() == reflect.Ptr {
			ty = t.Elem()
		} else {
			ty = t
		}
		msgT := reflect.New(ty)
		msgd := msgT.Interface()

		err = p.en.DecodeWithMaxLength(bytes.NewReader(msg.Payload), msgd)
		//
		value, ok := p.pending.Load(msg.ID)
		if ok {
			p.pending.Delete(msg.ID)
			reply := value.(chan interface{})
			select {
			case <-p.closing:
				return
			case reply <- msgd:
			}
			continue
		}
		handler, ok := MsgHanders[msg.Code]
		if !ok {
			returnFun(fmt.Errorf("Unknown message handler %s, %s", msg.String(), pe.GetID().String()))
			return
		}
		err = handler(msg.ID, msgd, pe)
		if err != nil {
			log.Warn(err.Error())
		}
	}
}

func (p *ConnMsgRW) readMsg(pe *Peer) (*Msg, error) {
	if p.closed.Load() {
		return nil, ErrConnClosed
	}
	dataHead := make([]byte, PacketSize)
	size, err := p.rw.Read(dataHead)
	if err == io.EOF {
		log.Debug("Base Stream closed by peer", "peer", pe.IDWithAddress())
		return nil, nil
	}
	if err != nil {
		log.Warn("Error reading from base stream", "peer", pe.IDWithAddress(), "error", err)
		return nil, err
	}
	if size != PacketSize {
		err = fmt.Errorf("Error message head size")
		log.Warn(err.Error(), "peer", pe.IDWithAddress())
		return nil, err
	}
	dataSize := binary.BigEndian.Uint64(dataHead)
	log.Debug("Receive message head", "peer", pe.IDWithAddress(), "size", dataSize)
	if dataSize > MaxMessageSize {
		return nil, fmt.Errorf("Too large message size: %d > %d", dataSize, MaxMessageSize)
	}
	msgData := make([]byte, dataSize)
	ctx, can := context.WithTimeout(context.Background(), HandleTimeout)
	defer can()

	ret := make(chan error)
	go func(ret chan error) {
		size, err = io.ReadFull(p.rw, msgData)
		select {
		case <-p.closing:
			return
		case ret <- err:
		}
	}(ret)

	select {
	case <-p.closing:
		return nil, ErrConnClosed
	case <-ctx.Done():
		return nil, fmt.Errorf("ConnMsgRW read message timeout:%s", pe.GetID())
	case err = <-ret:
	}

	if err == io.EOF {
		log.Debug("Base Stream closed by peer", "peer", pe.IDWithAddress())
		return nil, nil
	}
	if err != nil {
		log.Warn("Error reading from long stream", "peer", pe.IDWithAddress(), "error", err)
		return nil, err
	}
	if uint64(size) != dataSize {
		err = fmt.Errorf("Receive error size message data")
		log.Warn(err.Error(), "peer", pe.IDWithAddress())
		return nil, err
	}
	msgIDBs := msgData[:MsgCodeSize]
	msgID := binary.BigEndian.Uint64(msgIDBs)
	msgCodeBs := msgData[MsgCodeSize : MsgCodeSize*2]
	msgCode := binary.BigEndian.Uint64(msgCodeBs)

	log.Debug("Receive message", "id", msgID, "code", msgCode, "peer", pe.IDWithAddress(), "size", size)

	return &Msg{
		ID:      msgID,
		Code:    msgCode,
		Payload: msgData[MsgCodeSize*2:],
	}, nil
}

func NewConnRW(stream network.Stream, en encoder.NetworkEncoding) *ConnMsgRW {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	conn := &ConnMsgRW{
		rw:      rw,
		en:      en,
		w:       make(chan *Msg),
		closing: make(chan struct{}),
		closed:  new(atomic.Bool),
	}
	return conn
}
