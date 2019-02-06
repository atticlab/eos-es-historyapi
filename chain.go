package main

import (
	"io/ioutil"
	"net/http"
	"bytes"
	"encoding/json"
	"errors"
)


const RemoteNode                   string = "http://eosbp-0.atticlab.net"


//returns info from node chain api
func getInfo() (*ChainGetInfoResult, error) {
	resp, err := http.Get(RemoteNode + "/v1/chain/get_info")
	if err != nil {
		return nil, err
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	result := new(ChainGetInfoResult)
	err = json.Unmarshal(bytes, &result)
	return result, err
}

//takes blockNum and transactionId as arguments
//retrieves block from node chain api
//searches requested transaction in retrieved block
//returns the trx->trx field contents in the correct format
func getTransactionFromBlock(blockNum json.RawMessage, txId string) (json.RawMessage, error) {
	var result json.RawMessage
	u := GetBlockParams { BlockNum: blockNum }
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(u)
	resp, err := http.Post(RemoteNode + "/v1/chain/get_block", "application/json", b)
	if err != nil {
		return result, err
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return result, err
	}
	var getBlockResult ChainGetBlockResult
	err = json.Unmarshal(bytes, &getBlockResult)
	if err != nil {
		return result, err
	}
	for _, trx := range getBlockResult.Transactions {
		var tmp interface{}
		err = json.Unmarshal(trx.Trx, &tmp)
		if err != nil {
			return result, err
		}
		if s, ok := tmp.(string); ok {
			if s != txId {
				continue
			}
			result, err := json.Marshal([]interface{}{0, s})
			return result, err
		} else {
			var resTrx TransactionFromBlock
			err = json.Unmarshal(trx.Trx, &resTrx)
			if err != nil {
				return result, err
			}
			if resTrx.Id != txId {
				continue
			}
			resTrx.Id = ""
			result, err := json.Marshal([]interface{}{1, resTrx})
			return result, err
		}
	}
	return result, errors.New("Transaction not found")
}