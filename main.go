package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"socketio/conn"
	"socketio/httppush"
	"socketio/models"
	"socketio/models/redis"
	"time"
)
func GinMiddleware(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip:=c.ClientIP()
		imie:=c.Query("imie")
		if imie==""{
			c.AbortWithStatus(602)
			return
		}
		//fmt.Println("IMIE"+imie)
		deviceInfo,_:= models.DeviceM.GetByImie(imie)
		if deviceInfo==nil{
			//fmt.Println("imie号不存在", deviceInfo)
			c.AbortWithStatus(601)
			return
		}
		//fmt.Println(deviceInfo)
		if deviceInfo.Ip!=ip{
			c.AbortWithStatus(603)
			//return
		}
		//c.Writer.Header().Set("Access-Control-Allow-Origin", "http://192.168.66.166:9090")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Content-Length, X-CSRF-Token, Token, session, Origin, Host, Connection, Accept-Encoding, Accept-Language, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Request.Header.Del("Origin")

		c.Next()
	}
}
func main() {
	router := gin.New()
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.OnConnect("/", func(s socketio.Conn) error {
		//s.SetContext("")
		fmt.Println("connected:", s.ID())
		return nil
	})
	server.OnEvent("/", "join", func(s socketio.Conn, imie string,group string) {
		if group!=""{
			s.Join(group)
		}
		context:=map[string]string{s.ID():imie}
		s.SetContext(context)
		conn.ImieMap.Store(imie,s.ID())
		conn.ConnectionMap.Store(s.ID(), s)
		s.Join(conn.GetDefaultGroup())//全局分组
		//s.Join(s.ID())
		//s.Emit(conn.GetDefaultEvent(), s.ID())//默认事件推送
		fmt.Println("join:imie-"+imie)
		//fmt.Println(conn.ConnectionMap)
	})
	server.OnEvent("/", "task-reply", func(s socketio.Conn,msgId string) {
		if msgId==""{
			fmt.Println("msgID参数缺失")
			return
		}
		context:=s.Context()
		switch context.(type) {
		case map[string]string:
			contextMap:=context.(map[string]string)
			imie:=contextMap[s.ID()]
			//fmt.Println("task-reply",imie)
			redis.RedisModel.Hdel("SinglePush-imie:"+imie,msgId)//消息确认接收 删除记录
		}
	})
	/*server.OnEvent("/", "bye", func(s socketio.Conn) string {
		fmt.Println("aaaa")
		last := s.Context().(string)
		s.Emit("bye", last)
		s.LeaveAll()
		//conn.ImieMap.Delete(s.ID())
		conn.ConnectionMap.Delete(s.ID())
		s.Close()
		return last
	})*/
	server.OnError("/", func(s socketio.Conn,e error) {
		now := time.Now()
		fmt.Println(now.Format("2006-01-02 15:04:05")+" meet error:", e)
	})
	server.OnDisconnect("/", func(s socketio.Conn, msg string) {
		context:=s.Context()
		switch context.(type) {
		case map[string]string:
			contextMap:=context.(map[string]string)
			conn.ImieMap.Delete(contextMap[s.ID()])
			fmt.Println("disconnect:imie-", contextMap[s.ID()])
		}
		conn.ConnectionMap.Delete(s.ID())
		fmt.Println("disconnect:connId-", s.ID())
		s.LeaveAll()
		s.Close()
	})

	go server.Serve()
	defer server.Close()
	httpPush:= httppush.NewPush(server)
	router.POST("/grouppush/*any", httpPush.GroupPush)
	router.POST("/singlepush/*any", httpPush.SinglePush)
	router.Use(GinMiddleware(""))
	router.GET("/socket.io/*any", gin.WrapH(server))
	router.POST("/socket.io/*any", gin.WrapH(server))
	fmt.Println("start...")
	//router.StaticFS("/public", http.Dir("../asset"))

	router.Run(":5050")
}