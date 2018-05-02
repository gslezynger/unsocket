package unsocket


import (
	"encoding/binary"
	"fmt"
	"os"
	"net"
	"errors"
	"time"
	"encoding/json"
	"sync"
	internal "github.com/gslezynger/unsocket/internal"
)



var (
	//error list
	ErrConnListen = errors.New("Error listening")
	ErrConnListenPackage = errors.New("Error receiving packet")
	ErrConnListenMAX = errors.New("Error listening, maximum number of clients reached")
	ErrConnSendNoClient = errors.New("Error sending data, no available clients")
	ErrConnSendTimeout  = errors.New("Error sending to client, timeout")
	ErrConnSendVerification = errors.New("Data sent was corrupted")
	ErrConnReceiveVerification = errors.New("Data received was corrupted")
	ErrInternal = errors.New("Internal Error ...")
	ErrDuplicateMessageId = errors.New("Incoming duplicate message")

	//max listening clients
	MAXCLIENTS = 2
	//max idmessage length ( change for extra guarantee of not repeating )
	MAXSIZE_IDMESSAGE = 10

	TIMEOUTRESPONSE = 5 * time.Minute
)
type Client struct {
	timeout   int    //timeout in seconds for send: default 1s
	broadcast bool   //sends to all clients or to first: default false
	path      string //path that socket will be created on: default /tmp
	encrypted bool   //encrypt or not package to be sent :default true
	sync.Mutex
	messages map[string]bool
}


type envelope struct {
	Messageid string `json:"messageid"`
	Shasum * string `json:"shasum"`
	Data []byte `json:"data"`
}


func NewClient() *Client {
	return &Client{timeout:1,broadcast:false,path:"/tmp",encrypted:true,messages:make(map[string]bool,0)}
}
func (f * Client)SetTimeout(timeout int) *Client{
	f.timeout = timeout
	return f
}
func (f * Client)SetBroadcast(broadcast bool)*Client {
	f.broadcast = broadcast
	return f
}
func (f * Client)SetPath(path string) *Client{
	f.path = path
	return f
}
func (f * Client)SetEncrypted(encrypted bool)*Client {
	f.encrypted = encrypted
	return f
}



func (f * Client)Send(name string,data  interface{}) (* []byte,error){
	//encapsulates data
	dataToSend,err := json.Marshal(data)
	if( err != nil){
		return nil,ErrInternal
	}

	var dataEnvelope envelope = envelope{Messageid:internal.Randomstring(MAXSIZE_IDMESSAGE),Data:dataToSend,Shasum:nil}

	//gets data before shasum
	dataBts,sTcheck,err := generateSendPackage(&dataEnvelope,f.encrypted)
	if(err != nil){
		return nil,err
	}

	v,err := getavailablesockets(name,f.timeout,f.broadcast)
	if( err != nil){
		return nil,err
	}
	for _,c := range *v {
		defer c.Close()
	}
	for i,c := range *v {
		err = send(c,dataBts,f.encrypted,sTcheck)
		if ( err != nil && !f.broadcast){
			return nil,err
		}else if(err != nil && i == (len(*v) - 1)){
			return nil,err
		}else if( err != nil){
			continue
		}

		response,err := waitResponse(c)
		if ( err != nil && !f.broadcast){
			return nil,err
		}else if(err != nil && i == (len(*v) - 1)){
			return nil,err
		}else if( err != nil){
			continue
		}
		return &response.Data,nil
	}

	return nil,nil
}
func (f * Client)Listen(name string,callback func(data * []byte,err error)*[]byte) error {

	path := getfirstunusedsocket(f.path,name)
	if( path == nil){
		return ErrConnListenMAX
	}
	connection, err := net.Listen("unix", *path)
	if( err != nil){
		fmt.Println(err.Error())
		return err
	}

	fmt.Print("********* unsocket ********** \n Listening on: ")
	fmt.Println(*path)
	fmt.Println("*************************** ;)")
	for {
		//accepts unix socket connection
		fd, err := connection.Accept()
		if err != nil {
			fmt.Println("Error accepting connection :" + err.Error())
			continue
		}

		go func(){
			//receives package
			btsAshasum,isencrypted,err := read(fd)
			if(err != nil){
				fmt.Println("Error reading data :" + err.Error())
				fd.Close()
				//continue
				return
			}
			//unmarshal packages
			rPacket,sTsend,sTcheck,sTcheckr,err := generateReceivePackage(btsAshasum,*isencrypted)
			if(err != nil){
				//chamar callback
				fd.Close()
				callback(nil,ErrConnListenPackage)
				//continue
				return
			}


			//writes shasum
			err = write(fd,&sTsend,false)
			if(err != nil){
				//chamar callback
				fd.Close()
				callback(nil,ErrInternal)
				//continue
				return
			}

			//checks shasum
			if( string(sTcheck) != string(sTcheckr)){
				fmt.Println("shasums not matching")
				callback(nil,ErrConnReceiveVerification)
				//continue
				return
			}
			//checks for repeated messages
			if( f.messages[rPacket.Messageid] ){
				fmt.Println("Message already received!")
				callback(nil,ErrDuplicateMessageId)
				//continue
				return
			}
			//adds message to received map
			f.Lock()
			f.messages[rPacket.Messageid] = true
			f.Unlock()

			resCall := callback(&rPacket.Data,nil)
			if( resCall == nil){
				fd.Close()
				//continue
				return
			}
			var dataEnvelope envelope = envelope{Messageid:rPacket.Messageid,Data:*resCall,Shasum:nil}

			//gets data before shasum
			dataBts,sTcheck2,err := generateSendPackage(&dataEnvelope,*isencrypted)
			if(err != nil){
				fmt.Println("erro gerando pacote de resposta!")
				fd.Close()
				//continue
				return
			}
			err = send(fd,dataBts,*isencrypted,sTcheck2)
			if( err != nil ){
				fmt.Print("Erro e não há muito q eu possa fazer ")
				fmt.Println(err.Error())
			}
			fd.Close()

		}()

	}
}

func waitResponse(fd net.Conn) (*envelope , error) {

	var canal_timeout chan bool = make(chan bool,1)
	var canal_data chan []interface{} = make(chan []interface{})
	var data []interface{}
	//timeout for response
	go func(){
		time.Sleep( TIMEOUTRESPONSE )
		canal_timeout <- true
	}()
	go func(){
		readChannel(fd,canal_data)
	}()

	select{
		case data = <-canal_data:
			break
		case <-canal_timeout:
			fd.Close()
			return nil,ErrConnSendTimeout
	}
	//receives package
	btsAshasum,isencrypted,err1 := data[0],data[1],data[2]
	if(err1 != nil){
		fmt.Println("Error reading data :" + (err1).(error).Error())

		return nil,err1.(error)
	}
	//unmarshal packages
	rPacket,sTsend,sTcheck,sTcheckr,err := generateReceivePackage(btsAshasum.(*[]byte),*(isencrypted.(*bool)))
	if(err != nil){
		return nil,err
	}

	//writes shasum
	err = write(fd,&sTsend,false)
	if(err != nil){
		return nil,err
	}

	//checks shasum
	if( string(sTcheck) != string(sTcheckr)){
		fmt.Println("shasums not matching")

		return nil,ErrConnReceiveVerification
	}
	return rPacket,nil
}
func send(c net.Conn,data * []byte,encrypted bool,originalShasum * string) error {
	//sends data with shasum
	err := write(c,data,encrypted)
	if(err != nil){
		return err
	}
	//receives shasum from receiver
	dataShaSum,_,err := read(c)
	if(err != nil){
		return err
	}
	if(string(*dataShaSum) != *originalShasum){
		return ErrConnSendVerification
	}

	return nil
}

func write(c net.Conn,data * []byte,encrypt bool) error {
	var sizebuf []byte = make([]byte,len(*data) + 3)
	var isencrypted uint8 = 0
	if( encrypt ){
		isencrypted = 1
	}
	//copy data to new array, leaving 3 bytes for size and encryption
	copy(sizebuf[3:],*data)

	//set the first two bytes to the size of the package
	binary.LittleEndian.PutUint16(sizebuf[:2],uint16(len(*data)))

	//set the third byte to 1 or 0 ( depending on whether it should or not be encrypted )
	sizebuf[2] = isencrypted


	n, err := c.Write(sizebuf)
	if err != nil {
		fmt.Println("Error writing size buff " + err.Error())
		return err
	}
	if ( n != len(sizebuf)){
		return fmt.Errorf("error writing packet!")
	}

	return nil
}
func readChannel(fd net.Conn,canal_data chan []interface{}) {
	btsAshasum,isencrypted,err := read(fd)

	canal_data <- []interface{}{btsAshasum,isencrypted,err}

}
func read(fd net.Conn) (*[]byte,*bool,error) {
	//TO-DO: TIMEOUT NO READ


	//recebe o tamanho do pacote
	sizebuf := make([]byte, 3)
	_, err := fd.Read(sizebuf)
	if err != nil {
		fmt.Println("error reading data" + err.Error())
		return nil,nil,err
	}
	var isencrypted bool = false
	size := binary.LittleEndian.Uint16(sizebuf[:2])
	if( sizebuf[2] == 1 ){
		isencrypted = true
	}else{
		isencrypted  = false
	}
	databuf := make([]byte,size)
	_, err = fd.Read(databuf)
	if err != nil {
		fmt.Println("error reading data 2" + err.Error())
		return nil,nil,err
	}



	return &databuf,&isencrypted,nil
}


func getfirstunusedsocket(folder string,name string) *string{
	var fulpath string = fmt.Sprintf("%s/.%s",folder,name)
	var path string = fulpath
	c, err := net.Dial("unix", fulpath)

	var count int = 1
	for err == nil && count <= MAXCLIENTS {
		c.Close()
		path = fmt.Sprintf("%s%d",fulpath,count)
		c, err = net.Dial("unix", path)
		count += 1
	}
	if( count > MAXCLIENTS ){
		return nil
	}
	os.Remove(path)

	return &path
}
func getavailablesockets(name string,timeout int,all bool) (*[]net.Conn,error) {
	var fulpath string = fmt.Sprintf("/tmp/.%s",name)
	var path = fulpath
	var dialer net.Dialer = net.Dialer{Timeout:time.Duration(timeout) * time.Second}
	var count int = 1

	var connections []net.Conn

	for count <= MAXCLIENTS {
		c, err := dialer.Dial("unix", path)
		path = fmt.Sprintf("%s%d",fulpath,count)
		count += 1
		if( err != nil){
			continue
		}
		connections = append(connections,c)
		if( all == false){
			return &connections,nil
		}

	}
	if( len(connections) == 0){
		return nil,ErrConnSendNoClient
	}

	return &connections,nil
}
func generateSendPackage(data * envelope,encrypted bool)(*[]byte,*string,error){
	//gets data before shasum
	btsBshasumdata,err := json.Marshal(*data)
	if(err != nil){
		return nil,nil,err
	}
	if(data.Shasum == nil){
		//sets shasum of envelope
		data.Shasum = internal.Shasum(&btsBshasumdata)
	}

	//gets data after shasum
	btsAshasumdata,err := json.Marshal(*data)
	if(err != nil){
		return nil,nil,err
	}
	//generates shasum before crypto
	shaSumToCheck := internal.Shasum(&btsAshasumdata)
	if( encrypted ){
		btne,err := internal.Encrypt(&btsAshasumdata)
		if( err != nil){
			return nil,nil,err
		}
		btsAshasumdata = make([]byte,len(*btne))
		sizecopied := copy(btsAshasumdata,*btne)
		if  sizecopied != len(*btne) {
			return nil,nil,ErrInternal
		}

	}
	return &btsAshasumdata,shaSumToCheck,nil
}
func generateReceivePackage(data * []byte,encrypted bool)(*envelope,[]byte,[]byte,[]byte,error){

	var dataBts []byte = *data

	//if data is encrypted, decrypt
	if( encrypted ){
		ddata,err := internal.Decrypt(data)
		if( err != nil){
			return nil,nil,nil,nil,err
		}
		dataBts = make([]byte,len(*ddata))
		if copy(dataBts,*ddata) != len(*ddata) {
			return nil,nil,nil,nil,ErrInternal
		}

	}

	//unmarshal response
	var receivedJSON envelope
	err := json.Unmarshal(dataBts,&receivedJSON)
	if( err != nil){
		//chamar callback
		fmt.Println("Error unmarshalling, invalid format")
		return nil,nil,nil,nil,err
	}


	//generates shasum to send back
	shasumToSend := internal.Shasum(&dataBts)


	//stores received shaaum
	var receivedShasum string = *receivedJSON.Shasum
	receivedJSON.Shasum = nil
	btsBshasum,err := json.Marshal(receivedJSON)
	if( err != nil){
		//chamar callback
		fmt.Println("Error marshalling, invalid format")
		return nil,nil,nil,nil,err
	}
	//generates shasum to check against received
	shasumToCheck := internal.Shasum(&btsBshasum)


	return &receivedJSON,[]byte(*shasumToSend),[]byte(*shasumToCheck),[]byte(receivedShasum),nil
}

