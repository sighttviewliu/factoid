// Copyright 2015 Factom Foundation
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

// This is a minimum wallet to be used to test the coin
// There isn't much in the way of interest in security 
// here, but rather provides a mechanism to create keys
// and sign transactions, etc.

package wallet

import (
    "fmt"
    "github.com/agl/ed25519"
    "math/rand"
    fct "github.com/FactomProject/factoid"
    "github.com/FactomProject/factoid/database"
)

// The wallet interface uses bytes.  This is because we want to 
// handle fixed length values in our maps and the database.  If
// we try to use strings, then the lengths vary based on encoding
// and that complicates the implementation without really making
// the interface more usable by developers.
type ISCWallet interface {

    // Returns the backing database for the wallet
    GetDB() database.IFDatabase 
    // Generate an Factoid Address
    GenerateAddress(name []byte, m int, n int) (fct.IAddress, error)
    // Get details for an address
    GetAddressDetailsAddr(addr []byte) IWalletEntry
    // Get a list of names for addresses.  Names are easier for people to use.
    GetAddressList() (names [][]byte, addresses []fct.IAddress)
    
    /** Transaction calls **/
    // Create a transaction.  This is just the bones, to which the
    // user must add inputs, outputs, and sign before submission.
    CreateTransaction() fct.ITransaction
    // Modify an input.  Used to back fill the transaction fee.
    UpdateInput(fct.ITransaction, int, fct.IAddress, uint64) error
    // Add an input to a transaction
    AddInput(fct.ITransaction, fct.IAddress, uint64) error
    // Add an output to a transaction
    AddOutput(fct.ITransaction, fct.IAddress, uint64) error
    // Add an Entry Credit output to a transaction.  Note that these are 
    // denominated in Factoids.  So you need the exchange rate to do this
    // properly.
    AddECOutput(fct.ITransaction, fct.IAddress, uint64) error
    // Validate a transaction.  Just checks that the inputs and outputs are
    // there and properly constructed.
    Validate(fct.ITransaction) (bool,error)
    // Checks that the signatures all validate.
    ValidateSignatures(fct.ITransaction) bool
    // Sign the inputs that have public keys to which we have the private 
    // keys.  In the future, we will allow transactions with partical signatures
    // to be sent to other people to complete the signing process.  This will
    // be particularly useful with multisig.
    SignInputs(fct.ITransaction) (bool, error)   // True if all inputs are signed
    // Get the exchange rate of Factoids per Entry Credit
    GetECRate() uint64
    
}

var factoshisPerEC uint64 = 100000

var oneSCW SCWallet

type SCWallet struct {
    ISCWallet
    db database.MapDB
    r *rand.Rand
}

var _ ISCWallet = (*SCWallet)(nil)

func (w *SCWallet) GetDB() database.IFDatabase {
    return &w.db
}

func (SCWallet) GetDBHash() fct.IHash {
    return fct.Sha([]byte("SCWallet"))
}

func (w *SCWallet) SignInputs(trans fct.ITransaction) (bool, error) {
    
    data,err := trans.MarshalBinarySig()    // Get the part of the transaction we sign
    if err != nil { return false, err }    
    
    var numSigs int = 0
    
    inputs  := trans.GetInputs()
    rcds    := trans.GetRCDs()
    for i,rcd := range rcds {
        rcd1, ok := rcd.(*fct.RCD_1)
        if ok {
            pub := rcd1.GetPublicKey()
            we := w.db.GetRaw([]byte(fct.W_ADDRESS_PUB_KEY),pub).(*WalletEntry)
            if we != nil {
                var pri [fct.SIGNATURE_LENGTH]byte
                copy(pri[:],we.private[0])
                bsig := ed25519.Sign(&pri,data)
                sig := new(fct.Signature)
                sig.SetSignature(0,bsig[:])
                sigblk := new(fct.SignatureBlock)
                sigblk.AddSignature(sig)
                trans.SetSignatureBlock(i,sigblk)
                numSigs += 1
            }
        }
    }
    return numSigs == len(inputs), nil
}

func (w *SCWallet) GetECRate() uint64 {
    return factoshisPerEC
}

func (w *SCWallet) GetAddressDetailsAddr(name []byte) IWalletEntry {
    return w.db.GetRaw([]byte("wallet.address.addr"),name).(IWalletEntry)
}

func (w *SCWallet) GenerateAddress(name []byte,m int, n int) (hash fct.IAddress, err error) {
    
    we := new(WalletEntry)
    
    nm := w.db.GetRaw([]byte(fct.W_NAME_HASH),name)
    if nm != nil {
        return nil, fmt.Errorf("Duplicate Name")
    }
    
    if m == 1 && n == 1 {
        // Get a public/private key pair
        pub,pri,err := w.generateKey()
        // Error, skip out.
        if err != nil { return nil, err  }
        // Make sure we have not generated this pair before;  Keep
        // generating until we have a unique pair.
        for w.db.GetRaw([]byte(fct.W_ADDRESS_PUB_KEY),pub) != nil {
            pub,pri,err = w.generateKey()
            if err != nil { return nil, err  }
        }
        
        we.AddKey(pub,pri)
        we.SetName(name)
        we.SetRCD(fct.NewRCD_1(pub))

        // If the name exists already, then we store this as the hash of the name.
        // If that exists, then we store it as the hash of the hash and so forth.
        // This way, we can get a list of addresses with the same name.
        //
        nm  := w.db.GetRaw([]byte(fct.W_NAME_HASH),name)
        switch {
            case nm == nil :       // New Name
                hash, _ = we.GetAddress()
                w.db.PutRaw([]byte(fct.W_ADDRESS_HASH),hash.Bytes(),we)
                w.db.PutRaw([]byte(fct.W_ADDRESS_PUB_KEY),pub,we)                
                w.db.PutRaw([]byte(fct.W_NAME_HASH),name,we)
            case nm != nil :       // Duplicate name.  We generate a new name, and recurse.
                return nil, fmt.Errorf("Should never get here!  This is disabled!")
                nh := fct.Sha(name)
                return w.GenerateAddress(nh.Bytes(),m, n)
            default :
                return nil, fmt.Errorf("Should never get here!  This isn't possible!")
        }
        
    } else {
        return nil, fmt.Errorf("Not this far yet!")
    }
    return
}

func (w *SCWallet) Init (a ...interface{}) {
    if w.r != nil { return }
    w.r = rand.New(rand.NewSource(13436523)) 
    w.db.Init()
}
    
func (w *SCWallet) Read(buf []byte) (int, error) {
    //if r==nil { r = rand.New(rand.NewSource(time.Now().Unix())) }
    w.Init()
    for i := range buf {
        buf[i] = byte(w.r.Int())
    }
    return len(buf), nil
}

func (w *SCWallet) generateKey() (public []byte,private []byte, err error){
    pub,pri,err := ed25519.GenerateKey(w)
    return pub[:], pri[:], err
}

func (w *SCWallet)  CreateTransaction() fct.ITransaction {
    return new(fct.Transaction)
}

func (w *SCWallet) getWalletEntry(bucket []byte,address fct.IAddress) (IWalletEntry, fct.IAddress, error){
    
    v := w.db.GetRaw([]byte(fct.W_ADDRESS_HASH),address.Bytes())
    if(v == nil) { return nil, nil, fmt.Errorf("Unknown address") }
    
    we := v.(*WalletEntry)
    
    adr, err := we.GetAddress()
    if err != nil { return nil, nil, err }
    
    return we, adr, nil
}

func (w *SCWallet) AddInput(trans fct.ITransaction, address fct.IAddress, amount uint64) error {
    we, adr, err := w.getWalletEntry([]byte(fct.W_ADDRESS_HASH), address)
    if err != nil { return err }
    
     trans.AddInput(fct.CreateAddress(adr),amount)
     trans.AddRCD(we.GetRCD())
     
     return nil
}     

func (w *SCWallet) UpdateInput(trans fct.ITransaction, index int, address fct.IAddress, amount uint64) error {
    
    we, adr, err := w.getWalletEntry([]byte(fct.W_ADDRESS_HASH), address)
    if err != nil { return err }
                                   
    in,err := trans.GetInput(index)
    if err != nil {return err}
    
    trans.GetRCDs()[index] = we.GetRCD()      // The RCD must match the (possibly) new input
    
    in.SetAddress(adr)
    in.SetAmount(amount)
 
    return nil
}     

func (w *SCWallet) AddOutput(trans fct.ITransaction, address fct.IAddress, amount uint64) error {
    
    _, adr, err := w.getWalletEntry([]byte(fct.W_ADDRESS_HASH), address)
    if err != nil { return err }
    
    trans.AddOutput(fct.CreateAddress(adr),amount)
    if err != nil {
        return err
    }
    return nil
}     
 
 func (w *SCWallet) AddECOutput(trans fct.ITransaction, address fct.IAddress, amount uint64) error {
    
    _, adr, err := w.getWalletEntry([]byte(fct.W_ADDRESS_HASH), address)
    if err != nil { return err }
    
    trans.AddECOutput(fct.CreateAddress(adr),amount)
    if err != nil {
        return err
    }
    return nil
}

func (w *SCWallet) Validate(trans fct.ITransaction) (bool,error){
    valid := trans.Validate()
    if valid == fct.WELL_FORMED { return true, nil }
    
    fmt.Println("Validation Failed: ",valid)
    
    return false, nil
}    
 
 func (w *SCWallet) ValidateSignatures(trans fct.ITransaction) bool{
     valid := trans.ValidateSignatures()
     return valid
 } 
 