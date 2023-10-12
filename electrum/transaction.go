package electrum

import (
	"context"
	"log"
	"sync"

	"golang.org/x/sync/errgroup"
)

// BroadcastTransaction sends a raw transaction to the remote server to
// be broadcasted on the server network.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-broadcast
func (s *Client) BroadcastTransaction(
	ctx context.Context,
	rawTx string,
) (string, error) {
	resp := &basicResp{}
	err := s.request(
		ctx,
		"blockchain.transaction.broadcast",
		[]interface{}{rawTx},
		&resp,
	)
	if err != nil {
		return "", err
	}

	return resp.Result, nil
}

// GetTransactionResp represents the response to GetTransaction().
type GetTransactionResp struct {
	Result *GetTransactionResult `json:"result"`
}

// GetTransactionResult represents the content of the result field in the response to GetTransaction().
type GetTransactionResult struct {
	Blockhash     string               `json:"blockhash"`
	Blocktime     uint64               `json:"blocktime"`
	Confirmations int32                `json:"confirmations"`
	Hash          string               `json:"hash"`
	Hex           string               `json:"hex"`
	Locktime      uint32               `json:"locktime"`
	Size          uint32               `json:"size"`
	Time          uint64               `json:"time"`
	TxID          string               `json:"txid"`
	Version       uint32               `json:"version"`
	Vin           []Vin                `json:"vin"`
	Vout          []Vout               `json:"vout"`
	Merkle        GetMerkleProofResult `json:"merkle,omitempty"` // For protocol v1.5 and up.
}

type DetailedTransaction struct {
	*GetTransactionResult
	Vin []VinWithPrevout `json:"vin"`
}

// Vin represents the input side of a transaction.
type Vin struct {
	Coinbase  string    `json:"coinbase"`
	ScriptSig ScriptSig `json:"scriptSig"`
	Sequence  uint32    `json:"sequence"`
	TxID      string    `json:"txid"`
	Vout      uint32    `json:"vout"`
}

type VinWithPrevout struct {
	*Vin
	Prevout *Vout `json:"prevout"`
}

// ScriptSig represents the signature script for that transaction input.
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// Vout represents the output side of a transaction.
type Vout struct {
	N            uint32       `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
	Value        float64      `json:"value"`
}

// ScriptPubKey represents the script of that transaction output.
type ScriptPubKey struct {
	Addresses []string `json:"addresses,omitempty"`
	Address   string   `json:"address,omitempty"`
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex,omitempty"`
	ReqSigs   uint32   `json:"reqSigs,omitempty"`
	Type      string   `json:"type"`
}

// GetTransaction gets the detailed information for a transaction.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get
func (s *Client) GetTransaction(
	ctx context.Context,
	txHash string,
) (*GetTransactionResult, error) {
	var resp GetTransactionResp

	if tx, ok := s.txCache.Load(txHash); ok {
		if result, ok := tx.(*GetTransactionResult); ok {
			s.logger.Infof("Tx %s found in cache", txHash)
			return result, nil
		} else if detailedTx, ok := tx.(*DetailedTransaction); ok {
			return detailedTx.GetTransactionResult, nil
		}
	}

	err := s.request(
		ctx,
		"blockchain.transaction.get",
		[]interface{}{txHash, true},
		&resp,
	)
	if err != nil {
		return nil, err
	}

	if resp.Result != nil && resp.Result.Confirmations > 6 {
		s.txCache.Store(txHash, resp.Result)
	}

	return resp.Result, nil
}

// GetRawTransaction gets a raw encoded transaction.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get
func (s *Client) GetRawTransaction(
	ctx context.Context,
	txHash string,
) (string, error) {
	var resp basicResp

	err := s.request(
		ctx,
		"blockchain.transaction.get",
		[]interface{}{txHash, false},
		&resp,
	)
	if err != nil {
		return "", err
	}

	return resp.Result, nil
}

// GetTransactionOutput gets the Vout from a transaction.
func (s *Client) GetTransactionOutput(
	ctx context.Context,
	txHash string,
	outputIndex uint32,
) (*Vout, error) {
	tx, err := s.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, err
	}

	return &tx.Vout[outputIndex], nil
}

// Details a transaction by adding Prevout to Vin.
func (s *Client) DetailTransaction(
	ctx context.Context,
	tx *GetTransactionResult,
) (*DetailedTransaction, error) {
	if _tx, ok := s.txCache.Load(tx.TxID); ok {
		if detailedTx, ok := _tx.(*DetailedTransaction); ok {
			s.logger.Infof("DetailedTx %s found in cache", tx.TxID)
			return detailedTx, nil
		}
	}
	detailedTx := DetailedTransaction{
		GetTransactionResult: tx,
		Vin:                  []VinWithPrevout{}, // empty now
	}
	mtx := sync.Mutex{}
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(20)
	for _, vin := range tx.Vin {
		vin := vin // copy vin
		eg.Go(func() error {
			prevout, err := s.GetTransactionOutput(
				ctx,
				vin.TxID,
				vin.Vout,
			)
			if err != nil {
				return err
			}
			s.logger.Debugf(
				"from tx %s vin.tx %s prevou address & value: %v %f",
				tx.TxID,
				vin.TxID,
				getAddressFromVout(*prevout),
				prevout.Value,
			)
			mtx.Lock()
			defer mtx.Unlock()
			detailedTx.Vin = append(
				detailedTx.Vin,
				VinWithPrevout{
					Vin:     &vin,
					Prevout: prevout,
				},
			)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	s.txCache.Store(tx.TxID, &detailedTx)

	return &detailedTx, nil
}

// GetMerkleProofResp represents the response to GetMerkleProof().
type GetMerkleProofResp struct {
	Result *GetMerkleProofResult `json:"result"`
}

// GetMerkleProofResult represents the content of the result field in the response to GetMerkleProof().
type GetMerkleProofResult struct {
	Merkle   []string `json:"merkle"`
	Height   uint64   `json:"block_height"`
	Position uint32   `json:"pos"`
}

// GetMerkleProof returns the merkle proof for a confirmed transaction.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-get-merkle
func (s *Client) GetMerkleProof(
	ctx context.Context,
	txHash string,
	height uint32,
) (*GetMerkleProofResult, error) {
	var resp GetMerkleProofResp

	err := s.request(
		ctx,
		"blockchain.transaction.get_merkle",
		[]interface{}{txHash, height},
		&resp,
	)
	if err != nil {
		return nil, err
	}

	return resp.Result, err
}

// GetHashFromPosition returns the transaction hash for a specific position in a block.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-id-from-pos
func (s *Client) GetHashFromPosition(
	ctx context.Context,
	height, position uint32,
) (string, error) {
	var resp basicResp

	err := s.request(
		ctx,
		"blockchain.transaction.id_from_pos",
		[]interface{}{height, position, false},
		&resp,
	)
	if err != nil {
		return "", err
	}

	return resp.Result, err
}

// GetMerkleProofFromPosResp represents the response to GetMerkleProofFromPosition().
type GetMerkleProofFromPosResp struct {
	Result *GetMerkleProofFromPosResult `json:"result"`
}

// GetMerkleProofFromPosResult represents the content of the result field in the response
// to GetMerkleProofFromPosition().
type GetMerkleProofFromPosResult struct {
	Hash   string   `json:"tx_hash"`
	Merkle []string `json:"merkle"`
}

// GetMerkleProofFromPosition returns the merkle proof for a specific position in a block.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-transaction-id-from-pos
func (s *Client) GetMerkleProofFromPosition(
	ctx context.Context,
	height, position uint32,
) (*GetMerkleProofFromPosResult, error) {
	var resp GetMerkleProofFromPosResp

	err := s.request(
		ctx,
		"blockchain.transaction.id_from_pos",
		[]interface{}{height, position, true},
		&resp,
	)
	if err != nil {
		return nil, err
	}

	return resp.Result, err
}

func getAddressFromVout(vout Vout) string {
	if vout.ScriptPubKey.Address != "" {
		return vout.ScriptPubKey.Address
	}

	if vout.ScriptPubKey.Addresses != nil &&
		len(vout.ScriptPubKey.Addresses) > 0 {
		return vout.ScriptPubKey.Addresses[0]
	}

	return ""
}

// find address in vin and vout and call fn
func findAddressFunc[E any](
	address string,
	inouts []E,
	fn func(elem E, index int) bool,
) {
	for i, inout := range inouts {
		log.Printf("findAddressFunc: %s from %T[%d]", address, inout, i)
		var vout *Vout
		if vin, ok := any(inout).(VinWithPrevout); ok {
			vout = vin.Prevout
		} else if _vout, ok := any(inout).(Vout); ok {
			vout = &_vout
		} else {
			continue
		}
		if vout != nil {
			if getAddressFromVout(*vout) == address {
				if fn(inout, i) {
					continue
				} else {
					break
				}
			}
		}
	}
}
