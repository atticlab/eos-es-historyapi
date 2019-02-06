package main

import (
	"encoding/json"
)


type ErrorWithCode struct {
	Error error
	Code int
}


type ErrorResult struct {
	Code    int `json:"code"`
	Message string `json:"message"`
}


type ChainGetInfoResult struct {
	ServerVersion            json.RawMessage `json:"server_version"`
	ChainId                  json.RawMessage `json:"chain_id"`
	HeadBlockNum             json.RawMessage `json:"head_block_num"`
	LastIrreversibleBlockNum json.RawMessage `json:"last_irreversible_block_num"`
	LastIrreversibleBlockId  json.RawMessage `json:"last_irreversible_block_id"`
	HeadBlockId              json.RawMessage `json:"head_block_id"`
	HeadBlockTime            json.RawMessage `json:"head_block_time"`
	HeadBlockProducer        json.RawMessage `json:"head_block_producer"`
	VirtualBlockCpuLimit     json.RawMessage `json:"virtual_block_cpu_limit"`
	VirtualBlockNetLimit     json.RawMessage `json:"virtual_block_net_limit"`
	BlockCpuLimit            json.RawMessage `json:"block_cpu_limit"`
	BlockNetLimit            json.RawMessage `json:"block_net_limit"`
	ServerVersionString      json.RawMessage `json:"server_version_string"`
}

type GetBlockParams struct {
	BlockNum json.RawMessage `json:"block_num_or_id"`
}

type ChainGetBlockResult struct {
	Transactions []struct {
		Status        json.RawMessage `json:"status"`
		CpuUsageUs    json.RawMessage `json:"cpu_usage_us"`
		NetUsageWords json.RawMessage `json:"net_usage_words"`
		Trx           json.RawMessage `json:"trx"`
	} `json:"transactions"`
}

type TransactionFromBlock struct {
	Id                             string `json:"id,omitempty"`
	Signatures            json.RawMessage `json:"signatures"`
	Compression           json.RawMessage `json:"compression"`
	PackedContextFreeData json.RawMessage `json:"packed_context_free_data"`
	PackedTrx             json.RawMessage `json:"packed_trx"`
}


//get_actions types
type GetActionsParams struct {
	AccountName string `json:"account_name"`
	Pos         *int64 `json:"pos,omitempty"`
	Offset      *int64 `json:"offset,omitempty"`
}

type Action struct {
	GlobalActionSeq  json.RawMessage `json:"global_action_seq"`
	AccountActionSeq          uint64 `json:"account_action_seq"`
	BlockNum         json.RawMessage `json:"block_num"`
	BlockTime        json.RawMessage `json:"block_time"`
	ActionTrace      json.RawMessage `json:"action_trace"`
}

type GetActionsResult struct {
	Actions                      []Action `json:"actions"`
	LastIrreversibleBlock json.RawMessage `json:"last_irreversible_block"`
}


//get_transaction types
type GetTransactionParams struct {
	Id           string `json:"id"`
}

type GetTransactionResult struct {
	Id                             string `json:"id"`
	Trx        map[string]json.RawMessage `json:"trx"`
	BlockTime             json.RawMessage `json:"block_time"`
	BlockNum              json.RawMessage `json:"block_num"`
	Traces                json.RawMessage `json:"traces"`
	LastIrreversibleBlock json.RawMessage `json:"last_irreversible_block"`
}


//get_key_accounts types
type GetKeyAccountsParams struct {
	PublicKey string `json:"public_key"`
}

type GetKeyAccountsResult struct {
	AccountNames []string `json:"account_names"`
}


//get_controlled_accounts types
type GetControlledAccountsParams struct {
	ControllingAccount string `json:"controlling_account"`
}

type GetControlledAccountsResult struct {
	ControlledAccounts []string `json:"controlled_accounts"`
}