// Copyright 2015 Factom Foundation
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package block

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
    fct "github.com/FactomProject/factoid"
)

type IFBlock interface {
	fct.IBlock
	GetChainID() fct.IHash
	MarshalTrans() ([]byte, error)
    AddCoinbase(fct.ITransaction) (bool, error)
	AddTransaction(fct.ITransaction) (bool, error)
	CalculateHashes()
    GetMerkleRoot() fct.IHash
    GetPrevBlock() fct.IHash
    SetPrevBlock([]byte) 
    GetPrevHash3() fct.IHash
    SetPrevHash3([]byte) 
	SetDBHeight(uint32)
	GetDBHeight() uint32
	SetExchRate(uint64)
	GetExchRate() uint64
	GetUTXOCommit() fct.IHash
	SetUTXOCommit([]byte) 
    GetTransactions() []fct.ITransaction
}

// FBlockHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
//
// https://github.com/FactomProject/FactomDocs/blob/master/factomDataStructureDetails.md#factoid-block
//
type FBlock struct {
	IFBlock
	//  ChainID         IHash           // ChainID.  But since this is a constant, we need not actually use space to store it.
	MerkleRoot fct.IHash // Merkle root of the Factoid transactions which accompany this block.
	PrevBlock  fct.IHash // Key Merkle root of previous block.
	PrevHash3  fct.IHash // Sha3 of the previous Factoid Block
	ExchRate   uint64   // Factoshis per Entry Credit
	DBHeight   uint32   // Directory Block height
	UTXOCommit fct.IHash // This field will hold a Merkle root of an array containing all unspent transactions.
	// Transaction count
	// body size
	transactions []fct.ITransaction // List of transactions in this block
}

var _ IFBlock = (*FBlock)(nil)

func (b *FBlock) GetTransactions() []fct.ITransaction {
    return b.transactions
}

func (b FBlock) GetNewInstance() fct.IBlock {
    return new(FBlock)
}

func (FBlock) GetDBHash() fct.IHash {
    return fct.Sha([]byte("FBlock"))
}

func (b *FBlock) GetHash() fct.IHash {
    data,err := b.MarshalBinary()
    if err != nil {
        fmt.Println(err)
        return nil
    }
    return fct.Sha(data)
}

func (b *FBlock) MarshalTrans() ([]byte, error) {
	var out bytes.Buffer
	for _, trans := range b.transactions {
		data, err := trans.MarshalBinary()
		if err != nil {
			return nil, err
		}
		out.Write(data)
		if err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// Write out the block
func (b *FBlock) MarshalBinary() ([]byte, error) {
	var out bytes.Buffer
	b.CalculateHashes()
	out.Write(fct.FACTOID_CHAINID)
    
    if b.MerkleRoot == nil {b.MerkleRoot = new(fct.Hash)}
    data, err := b.MerkleRoot.MarshalBinary()
	if err != nil {
		return nil, err
	}
	out.Write(data)
    
    if b.PrevBlock == nil {b.PrevBlock = new(fct.Hash)}
	data, err = b.PrevBlock.MarshalBinary()
	if err != nil {
		return nil, err
	}
	out.Write(data)
    
    if b.PrevHash3 == nil {b.PrevHash3 = new(fct.Hash)}
    data, err = b.PrevHash3.MarshalBinary()
	if err != nil {
		return nil, err
	}
	out.Write(data)
    
	binary.Write(&out, binary.BigEndian, uint64(b.ExchRate))
	binary.Write(&out, binary.BigEndian, uint32(b.DBHeight))
    
    if b.UTXOCommit == nil {b.UTXOCommit = new(fct.Hash)}
    data, err = b.UTXOCommit.MarshalBinary()
	if err != nil {
		return nil, err
	}
	out.Write(data)
    
	binary.Write(&out, binary.BigEndian, uint64(len(b.transactions)))

	transdata, err := b.MarshalTrans()                           // first get trans data
	binary.Write(&out, binary.BigEndian, uint64(len(transdata))) // write out its length
	out.Write(transdata)                                         // write out trans data

	return out.Bytes(), nil
}

// UnmarshalBinary assumes that the Binary is all good.  We do error
// out if there isn't enough data, or the transaction is too large.
func (b *FBlock) UnmarshalBinaryData(data []byte) ([]byte, error) {
	if bytes.Compare(data[:fct.ADDRESS_LENGTH], fct.FACTOID_CHAINID[:]) != 0 {
		return nil, fmt.Errorf("Block does not begin with the Factoid ChainID")
	}
	data = data[32:]
	
	b.MerkleRoot = new(fct.Hash)
	data, err := b.MerkleRoot.UnmarshalBinaryData(data)
	if err != nil {
		return nil, err
	}

	b.PrevBlock = new(fct.Hash)
	data, err = b.PrevBlock.UnmarshalBinaryData(data)
	if err != nil {
		return nil, err
	}
	
	b.PrevHash3 = new(fct.Hash)
	data, err = b.PrevHash3.UnmarshalBinaryData(data)
	if err != nil {
		return nil, err
	}

	b.ExchRate, data = binary.BigEndian.Uint64(data[0:8]), data[8:]
	b.DBHeight, data = binary.BigEndian.Uint32(data[0:4]), data[4:]

	b.UTXOCommit = new(fct.Hash)
	data, err = b.UTXOCommit.UnmarshalBinaryData(data)
	if err != nil {
		return nil, err
	}

	cnt, data := binary.BigEndian.Uint64(data[0:8]), data[8:]

	data = data[8:] // Just skip the size... We don't really need it.

	b.transactions = make([]fct.ITransaction, cnt, cnt)
	for i := uint64(0); i < cnt; i++ {
        trans := new(fct.Transaction)
        data,err = trans.UnmarshalBinaryData(data)
        if err != nil {
            fct.Prtln("Failed to unmarshal a transaction in block.",err)
            return nil, fmt.Errorf("Failed to unmarshal a transaction in block.\n%s",b.String())
        }
		b.transactions[i] = trans
	}
	return data, nil
}

func (b *FBlock) UnmarshalBinary(data []byte) (err error) {
	data, err = b.UnmarshalBinaryData(data)
	return err
}

// Tests if the transaction is equal in all of its structures, and
// in order of the structures.  Largely used to test and debug, but
// generally useful.
func (b1 *FBlock) IsEqual(block fct.IBlock) []fct.IBlock {

	b2, ok := block.(*FBlock)

	if !ok || // Not the right kind of IBlock
        b1.ExchRate != b2.ExchRate ||
        b1.DBHeight != b2.DBHeight {
            r := make([]fct.IBlock,0,3)
            return append(r,b1)
        }
        
    r := b1.MerkleRoot.IsEqual(b2.MerkleRoot)
    if r != nil {
        return append(r,b1)
    }
    r = b1.PrevBlock.IsEqual(b2.PrevBlock)
    if r != nil {
        return append(r,b1)
    }
    r = b1.PrevHash3.IsEqual(b2.PrevHash3) 
    if r != nil {
        return append(r,b1)
    }
    r = b1.UTXOCommit.IsEqual(b2.UTXOCommit) 
    if r != nil {
        return append(r,b1)
    }
	

	for i, trans := range b1.transactions {
		r := trans.IsEqual(b2.transactions[i]) 
		if r != nil {
            return append(r,b1)
		}
	}

	return nil
}
func (b *FBlock) GetChainID() fct.IHash {
    h := new(fct.Hash)
    h.SetBytes(fct.FACTOID_CHAINID)
    return h
}
func (b *FBlock) GetMerkleRoot() fct.IHash {
    return b.MerkleRoot
}
func (b *FBlock) GetPrevBlock() fct.IHash {
    return b.PrevBlock
}
func (b *FBlock) SetPrevBlock(hash []byte) {
    h := fct.NewHash(hash)
    b.PrevBlock= h
}
func (b *FBlock) GetPrevHash3() fct.IHash {
    return b.PrevHash3
}
func (b *FBlock) SetPrevHash3(hash[]byte)  {
    b.PrevHash3.SetBytes(hash)
}
func (b *FBlock) GetUTXOCommit() fct.IHash {
    return b.UTXOCommit
}
func (b *FBlock) SetUTXOCommit(hash[]byte) {
    b.UTXOCommit.SetBytes(hash)
}
func (b *FBlock) CalculateHashes() {
}
func (b *FBlock) SetDBHeight(dbheight uint32) {
	b.DBHeight = dbheight
}
func (b *FBlock) GetDBHeight() uint32 {
	return b.DBHeight
}
func (b *FBlock) SetExchRate(rate uint64) {
	b.ExchRate = rate
}
func (b *FBlock) GetExchRate() uint64 {
	return b.ExchRate
}

func (b FBlock) Validate() (bool, error) {
	for _, trans := range b.transactions {
		valid := trans.Validate()
		if valid != fct.WELL_FORMED {
			return false, fmt.Errorf("Block contains invalid transactions:\n%s",valid)
		}
	}

	// Need to check balances are all good.

	// Save what we got for our hashes
	mr := b.MerkleRoot
	pb := b.PrevBlock
	ph := b.PrevHash3

	// Recalculate the hashes
	b.CalculateHashes()

	// Make sure nothing changes.  If something did, this block is bad.
	return mr == b.MerkleRoot && pb == b.PrevBlock && ph == b.PrevHash3, nil
}

// Add the first transaction of a block.  This transaction makes the 
// payout to the servers, so it has no inputs.   This transaction must
// be deterministic so that all servers will know and expect its output.
func (b *FBlock) AddCoinbase(trans fct.ITransaction) (bool, error) {
    if len(b.transactions)              != 0 ||
       len(trans.GetInputs())           != 0 || 
       len(trans.GetECOutputs())           != 0 ||
       len(trans.GetRCDs())             != 0 ||
       len(trans.GetSignatureBlocks())  != 0 {
        return false, fmt.Errorf("Cannot have inputs or EC outputs in the coinbase.")
    }

    // TODO Add check here for the proper payouts.
    
    b.transactions = append(b.transactions, trans)
    return true, nil
}
    

// Add a transaction to the Facoid block. If there is an error,
// then the transaction can be discarded.  If it returns true,
// then the transaction was added, if false it was not.
func (b *FBlock) AddTransaction(trans fct.ITransaction) (bool, error) {
	// These tests check that the Transaction itself is valid.  If it
	// is not internally valid, it never will be valid.
	valid := trans.Validate()
	if valid != fct.WELL_FORMED {
		return false, fmt.Errorf("Invalid Transaction: %s",valid)
	}

	// These checks may pass in the future

	// Check against address balances

	b.transactions = append(b.transactions, trans)
	return true, nil
}

func (b FBlock) String() string {
    txt,err := b.MarshalText()
    if err != nil {return err.Error() }
    return string(txt)
}

// Marshal to text.  Largely a debugging thing.
func (b FBlock) MarshalText() (text []byte, err error) {
	var out bytes.Buffer

	out.WriteString("Transaction Block\n")
	out.WriteString("  ChainID: ")
	out.WriteString(hex.EncodeToString(fct.FACTOID_CHAINID))
    if b.MerkleRoot == nil { b.MerkleRoot = new (fct.Hash) }
    out.WriteString("\n  MerkleRoot: ")
    out.WriteString(b.MerkleRoot.String())
    if b.PrevBlock == nil { b.PrevBlock = new (fct.Hash) }
    out.WriteString("\n  PrevBlock: ")
	out.WriteString(b.PrevBlock.String())
    if b.PrevHash3 == nil { b.PrevHash3 = new (fct.Hash) }
    out.WriteString("\n  PrevHash3: ")
	out.WriteString(b.PrevHash3.String())
	out.WriteString("\n  ExchRate: ")
	fct.WriteNumber64(&out, b.ExchRate)
	out.WriteString("\n  DBHeight: ")
	fct.WriteNumber32(&out, b.DBHeight)
    if b.UTXOCommit == nil { b.UTXOCommit = new (fct.Hash) }
    out.WriteString("\n  UTXOCommit: ")
	out.WriteString(b.UTXOCommit.String())
	out.WriteString("\n  Number Transactions: ")
	fct.WriteNumber64(&out, uint64(len(b.transactions)))
	transdata, err := b.MarshalTrans()
	if err != nil {
		return out.Bytes(), err
	}
	out.WriteString("\n  Body Size: ")
	fct.WriteNumber64(&out, uint64(len(transdata)))
	out.WriteString("\n")
	for _, trans := range b.transactions {
		txt, err := trans.MarshalText()
		if err != nil {
			return out.Bytes(), err
		}
		out.Write(txt)
	}
	return out.Bytes(), nil
}

/**************************
 * Helper Functions
 **************************/

func NewFBlock(ExchRate uint64, DBHeight uint32) IFBlock {
	scb := new(FBlock)
    scb.MerkleRoot = new (fct.Hash)
    scb.PrevBlock  = new (fct.Hash)
    scb.PrevHash3  = new (fct.Hash)
    scb.UTXOCommit = new (fct.Hash)
	scb.ExchRate   = ExchRate
	scb.DBHeight   = DBHeight
	return scb
}