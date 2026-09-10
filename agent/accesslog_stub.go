//go:build !windows

package main

func readRDPLogons(afterRecord int64) ([]rdpEvent, error) {
	_ = afterRecord
	return nil, nil
}
