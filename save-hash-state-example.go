package main

import (
	"bytes"
	"context"
	"crypto"
	_ "crypto/md5"
	"encoding"
	"fmt"
	"time"
)

type State struct {
	Offset int64
	Data   []byte
}

func MD5(data []byte) []byte {
	hash := crypto.MD5.New()
	hash.Write(data)
	return hash.Sum(nil)
}

func StartMD5(ctx context.Context, data []byte, state *State) (<-chan *State, <-chan []byte) {
	chState := make(chan *State)
	chChecksum := make(chan []byte)

	go func() {
		defer func() {
			close(chState)
			close(chChecksum)
		}()

		l := int64(len(data))
		i := int64(0)
		hash := crypto.MD5.New()

		// Restore previous hash state.
		if state != nil {
			i = state.Offset
			m := hash.(encoding.BinaryUnmarshaler)
			m.UnmarshalBinary(state.Data)
		}

		for {
			select {
			case <-ctx.Done():
				// Stopped by user.
				fmt.Printf("Stopped: %v\n", ctx.Err())
				// Save state of hash.
				m := hash.(encoding.BinaryMarshaler)
				buf, _ := m.MarshalBinary()
				chState <- &State{Offset: int64(i), Data: buf}
				return
			default:
				if i < l {
					hash.Write([]byte{data[i]})
					i++
				} else {
					chChecksum <- hash.Sum(nil)
					return
				}
				time.Sleep(time.Millisecond * 200)
			}
		}
	}()

	return chState, chChecksum
}

func main() {
	data := []byte("Hello World!")

	// 1. Write all data to hash to compute checksum(sync).
	checksum1 := MD5(data)
	fmt.Printf("MD5 checksum 1: %X\n", checksum1)

	// 2. Start a goroutine to compute checksum(async).
	// Use a timeout to emulate user stopping hash,
	// and get the saved hash state.
	ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*300)
	chState, chChecksum := StartMD5(ctx, data, nil)
	var state *State

LOOP:
	for {
		select {
		case state = <-chState:
			fmt.Printf("MD5 state:\n")
			fmt.Printf("Offset: %d\nData: %X\n", state.Offset, state.Data)
			break LOOP
		case checksum2 := <-chChecksum:
			fmt.Printf("MD5 checksum 2: %X\n", checksum2)
			break LOOP
		}
	}

	// 3. Start a goroutine to resume computing checksum with stored hash state(async).
	ctx = context.Background()
	chState, chChecksum = StartMD5(ctx, data, state)

LOOP2:
	for {
		select {
		case state := <-chState:
			fmt.Printf("MD5 state:\n")
			fmt.Printf("Offset: %d\nData: %X\n", state.Offset, state.Data)
			break LOOP2
		case checksum2 := <-chChecksum:
			fmt.Printf("MD5 checksum 2: %X\n", checksum2)
			result := bytes.Compare(checksum1, checksum2)
			if result == 0 {
				fmt.Printf("checksum 1 == checksum 2\n")
			} else {
				fmt.Printf("checksum 1 != checksum 2\n")
			}
			break LOOP2
		}
	}
}
