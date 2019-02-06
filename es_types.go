package main

import (
	"encoding/json"
)


type ActionTrace struct {
	Receipt struct {
		Receiver       json.RawMessage `json:"receiver"`
		ActDigest      json.RawMessage `json:"act_digest"`
		GlobalSequence json.RawMessage `json:"global_sequence"`
		RecvSequence   json.RawMessage `json:"recv_sequence"`
		AuthSequence   json.RawMessage `json:"auth_sequence"`
		CodeSequence   json.RawMessage `json:"code_sequence"`
		AbiSequence    json.RawMessage `json:"abi_sequence"`
	} `json:"receipt"`
	Act struct {
		Account       json.RawMessage `json:"account"`
		Name          json.RawMessage `json:"name"`
		Authorization json.RawMessage `json:"authorization"`
		Data          json.RawMessage `json:"data"`
		HexData       json.RawMessage `json:"hex_data"`
	} `json:"act"`
	ContextFree      json.RawMessage `json:"context_free"`
	Elapsed          json.RawMessage `json:"elapsed"`
	Console          json.RawMessage `json:"console"`
	TrxId                     string `json:"trx_id"`
	BlockNum         json.RawMessage `json:"block_num"`
	BlockTime        json.RawMessage `json:"block_time"`
	ProducerBlockId  json.RawMessage `json:"producer_block_id"`
	AccountRamDeltas json.RawMessage `json:"account_ram_deltas"`
	Except           json.RawMessage `json:"except"`
}


type Transaction struct {
	TrxId                 json.RawMessage `json:"trx_id"`
	Irreversible          json.RawMessage `json:"irreversible"`
	BlockId               json.RawMessage `json:"block_id"`
	BlockNum              json.RawMessage `json:"block_num"`
	Implicit              json.RawMessage `json:"implicit"`
	Scheduled             json.RawMessage `json:"scheduled"`
	SigningKeys           json.RawMessage `json:"signing_keys"`
	Signatures            json.RawMessage `json:"signatures"`
	Accepted              json.RawMessage `json:"accepted"`
	MaxNetUsageWords      json.RawMessage `json:"max_net_usage_words"`
	MaxCpuUsageMs         json.RawMessage `json:"max_cpu_usage_ms"`
	TransactionExtensions json.RawMessage `json:"transaction_extensions"`
	Expiration            json.RawMessage `json:"expiration"`
	DelaySec              json.RawMessage `json:"delay_sec"`
	RefBlockNum           json.RawMessage `json:"ref_block_num"`
	RefBlockPrefix        json.RawMessage `json:"ref_block_prefix"`
	Actions               json.RawMessage `json:"actions"`
	ContextFreeActions    json.RawMessage `json:"context_free_actions"`
	ContextFreeData       json.RawMessage `json:"context_free_data"`
}


type TransactionTraceActionTrace struct {
	Receipt          json.RawMessage `json:"receipt"`
	Act struct {
		Account                  string `json:"account"`
		Name                     string `json:"name"`
		Authorization   json.RawMessage `json:"authorization"`
		Data interface{} `json:"data"`
		HexData                  string `json:"hex_data,omitempty"`
	} `json:"act"`
	ContextFree      json.RawMessage `json:"context_free"`
	Elapsed          json.RawMessage `json:"elapsed"`
	Console          json.RawMessage `json:"console"`
	TrxId            json.RawMessage `json:"trx_id"`
	BlockNum         json.RawMessage `json:"block_num"`
	BlockTime        json.RawMessage `json:"block_time"`
	ProducerBlockId  json.RawMessage `json:"producer_block_id"`
	AccountRamDeltas json.RawMessage `json:"account_ram_deltas"`
	Except           json.RawMessage `json:"except"`
	InlineTraces []TransactionTraceActionTrace `json:"inline_traces"`
}


type TransactionTrace struct {
	Id              json.RawMessage `json:"id"`
	BlockNum        json.RawMessage `json:"block_num"`
	BlockTime       json.RawMessage `json:"block_time"`
	ProducerBlockId json.RawMessage `json:"producer_block_id"`
	Receipt map[string]json.RawMessage `json:"receipt"`
	Elapsed         json.RawMessage `json:"elapsed"`
	NetUsage        json.RawMessage `json:"net_usage"`
	Scheduled       json.RawMessage `json:"scheduled"`
	ActionTraces []TransactionTraceActionTrace `json:"action_traces"`
	Except json.RawMessage `json:"except"`
}


type Account struct {
	Name             string `json:"name"`
	Creator json.RawMessage `json:"creator"`
	PubKeys json.RawMessage `json:"pub_keys"`
	AccountControls [] struct {
		Name       json.RawMessage `json:"name"`
		Permission json.RawMessage `json:"permission"`
	} `json:"account_controls"`
	Abi               json.RawMessage `json:"abi"`
	AccountCreateTime json.RawMessage `json:"account_create_time"`
}