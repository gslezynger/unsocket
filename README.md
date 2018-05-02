# Intraserver Communication in Golang

[![Build Status](https://travis-ci.org/gslezynger/unsocket.svg?branch=master)](https://travis-ci.org/gslezynger/unsocket)

Supports:

- Bidirectional communication.
- Multiple listeners
- Encryted messages
- Feedback of delivery 
- Retrial to next listener


## Installation

Install:

```shell
go get -u github.com/gslezynger/unsocket
```

Import:

```go
import "github.com/gslezynger/unsocket"
```

## Server Usage Example

```go

//changes number of maximum clientes listening
unsocket.MAXCLIENTS = 2 
  
//listen passing: 'name of service' and a callback function
err := unsocket.NewClient().Listen("payment.services",func(data  * []byte,err error)*[]byte{
		//callback whenever service receives message
		
		//check if error ocurred during package receival
	    if(err != nil){
			fmt.Println("Erro ao receber pacote : " + err.Error())
			return nil
		}



        //return nil or *[]byte array of response 
		return nil
	})
 
if( err != nil ){
    fmt.Println("Error starting server: " + err.Error())
}
```

## Client Usage Example

```go
//creates new instance of unsocket Client
client := unsocket.NewClient()
 
//sets whether you want package to be encrypted
client.SetEncrypted(true)
 
//set timeout of sending messages
client.SetTimeout(1)
 
//sets if a message is not delivered, to deliver to the next client
client.SetBroadcast(true)
 
type Message struct {
	Name string 
}
 
var mesg Message
mesg.Name = "Dunha"

//send passing: 'name of service' and anything to be sent
	response,err := client.Send("payment.services",mesg)
	if( err != nil){
		fmt.Println("Error sending package :" + err.Error())
		return
	}
	fmt.Print(string(*response))

```