package internal

import (
	"crypto/aes"
	"fmt"
	"crypto/cipher"
	"io"
	"crypto/sha1"
	"encoding/hex"
	"crypto/rand"
	"errors"
)
var ErrCrypt = errors.New("Invalid data to decrypt / encrypt")
const key = `5991A1AEFC438519EDFC5ED1A41A6&11`

func Encrypt(data * []byte)(*[]byte,error) {
	//creating a block cipher
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		fmt.Println(err.Error())
		return nil,err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fmt.Println(err.Error())
		return nil,err
	}
	nonce := make([]byte,gcm.NonceSize())
	if _,err = io.ReadFull(rand.Reader,nonce);err != nil {
		return nil,err
	}
	var encrypteddata []byte
	encrypteddata = gcm.Seal( nonce,nonce,*data,nil)

	//var encrypteddata []byte


	//block.Encrypt(encrypteddata,*data)

	return &encrypteddata,nil
}
func Decrypt(data * []byte)(*[]byte,error) {

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		//fmt.Println(err.Error())
		return nil,err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		//fmt.Println(err.Error())
		return nil,err
	}

	nonceSize := gcm.NonceSize()
	if len(*data) < nonceSize {
		return nil,ErrCrypt
	}


	nonce,ddata := (*data)[:nonceSize],(*data)[nonceSize:]


	var result []byte
	result,err = gcm.Open(nil,nonce,ddata,nil)
	if ( err != nil){
		fmt.Println(err.Error())
		return nil,err
	}

	return &result,nil
	//block, err := aes.NewCipher([]byte(key))
	//if err != nil {
	//	fmt.Println(err.Error())
	//}
	//var decrypteddata []byte
	//
	//block.Decrypt(decrypteddata,*data)
	//
	//return &decrypteddata,nil
}
func Shasum(data * []byte) *string {
	algorithm := sha1.New()
	algorithm.Write(*data)
	var str string = hex.EncodeToString(algorithm.Sum(nil))

	return &str
}
