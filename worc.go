package worc

import (
	"fmt"
	"log"
	"reflect"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	wonaming "github.com/wothing/wonaming/etcd"
)

var serviceConns = newSafeMap()

// StartServiceConns start grpc conns with balancer
func StartServiceConns(address string, serviceList []string) {
	for _, serviceName := range serviceList {
		go func(name string) {
			r := wonaming.NewResolver(name)
			b := grpc.RoundRobin(r)

			conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBalancer(b))
			if err != nil {
				log.Printf(`connect to '%s' service failed: %v`, name, err)
			}
			serviceConns.Set(name, conn)
		}(serviceName)
	}
}

// CloseServiceConns close all established conns
func CloseServiceConns() {
	for _, conn := range serviceConns.List() {
		conn.Close()
	}
}

// CallRPC is helper func that make life easier
// ctx: context
// client: grpc client
// serviceName: name of service
// metod: method name that you want to use
// req: grpc request
func CallRPC(ctx context.Context, client interface{}, serviceName string, method string, req interface{}) (ret interface{}, err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("call RPC '%s' error: %v", method, x)
		}
	}()

	conn := serviceConns.Get(serviceName)
	if conn == nil {
		return nil, fmt.Errorf("service conn '%s' not found", serviceName)
	}

	// get NewServiceClient's reflect.Value
	vClient := reflect.ValueOf(client)
	var vParameter []reflect.Value
	vParameter = append(vParameter, reflect.ValueOf(conn))

	// c[0] is serviceServer reflect.Value
	c := vClient.Call(vParameter)

	// rpc param
	v := make([]reflect.Value, 2)
	v[0] = reflect.ValueOf(ctx)
	v[1] = reflect.ValueOf(req)

	// rpc method call
	f := c[0].MethodByName(method)
	resp := f.Call(v)
	if !resp[1].IsNil() {
		return nil, resp[1].Interface().(error)
	}
	return resp[0].Interface(), nil
}
