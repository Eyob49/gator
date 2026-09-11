package main

import (
	"reflect"
	"testing"
)

func TestNewCommandsRegistersAggToTheAggregatorHandler(t *testing.T) {
	cmds := newCommands()

	aggHandler, ok := cmds.handlers["agg"]
	if !ok {
		t.Fatal("agg command should be registered")
	}

	if reflect.ValueOf(aggHandler).Pointer() != reflect.ValueOf(handlerAgg).Pointer() {
		t.Fatal("agg should point at handlerAgg")
	}
}
