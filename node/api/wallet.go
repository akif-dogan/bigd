package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	mnemonics "gitlab.com/NebulousLabs/entropy-mnemonics"
	"gitlab.com/NebulousLabs/errors"

	"go.thebigfile.com/bigd/crypto"
	"go.thebigfile.com/bigd/modules"
	"go.thebigfile.com/bigd/types"
)

type (
	// WalletGET contains general information about the wallet.
	WalletGET struct {
		Encrypted  bool              `json:"encrypted"`
		Height     types.BlockHeight `json:"height"`
		Rescanning bool              `json:"rescanning"`
		Unlocked   bool              `json:"unlocked"`

		ConfirmedSiacoinBalance     types.Currency `json:"confirmedsiacoinbalance"`
		UnconfirmedOutgoingSiacoins types.Currency `json:"unconfirmedoutgoingsiacoins"`
		UnconfirmedIncomingSiacoins types.Currency `json:"unconfirmedincomingsiacoins"`

		SiacoinClaimBalance types.Currency `json:"siacoinclaimbalance"`
		SiafundBalance      types.Currency `json:"siafundbalance"`

		DustThreshold types.Currency `json:"dustthreshold"`
	}

	// WalletAddressGET contains an address returned by a GET call to
	// /wallet/address.
	WalletAddressGET struct {
		Address types.UnlockHash `json:"address"`
	}

	// WalletAddressesGET contains the list of wallet addresses returned by a
	// GET call to /wallet/addresses.
	WalletAddressesGET struct {
		Addresses []types.UnlockHash `json:"addresses"`
	}

	// WalletInitPOST contains the primary seed that gets generated during a
	// POST call to /wallet/init.
	WalletInitPOST struct {
		PrimarySeed string `json:"primaryseed"`
	}

	// WalletSiacoinsPOST contains the transaction sent in the POST call to
	// /wallet/siacoins.
	WalletSiacoinsPOST struct {
		Transactions   []types.Transaction   `json:"transactions"`
		TransactionIDs []types.TransactionID `json:"transactionids"`
	}

	// WalletSiafundsPOST contains the transaction sent in the POST call to
	// /wallet/siafunds.
	WalletSiafundsPOST struct {
		Transactions   []types.Transaction   `json:"transactions"`
		TransactionIDs []types.TransactionID `json:"transactionids"`
	}

	// WalletSignPOSTParams contains the unsigned transaction and a set of
	// inputs to sign.
	WalletSignPOSTParams struct {
		Transaction types.Transaction `json:"transaction"`
		ToSign      []crypto.Hash     `json:"tosign"`
	}

	// WalletSignPOSTResp contains the signed transaction.
	WalletSignPOSTResp struct {
		Transaction types.Transaction `json:"transaction"`
	}

	// WalletSeedsGET contains the seeds used by the wallet.
	WalletSeedsGET struct {
		PrimarySeed        string   `json:"primaryseed"`
		AddressesRemaining int      `json:"addressesremaining"`
		AllSeeds           []string `json:"allseeds"`
	}

	// WalletSweepPOST contains the coins and funds returned by a call to
	// /wallet/sweep.
	WalletSweepPOST struct {
		Coins types.Currency `json:"coins"`
		Funds types.Currency `json:"funds"`
	}

	// WalletTransactionGETid contains the transaction returned by a call to
	// /wallet/transaction/:id
	WalletTransactionGETid struct {
		Transaction modules.ProcessedTransaction `json:"transaction"`
	}

	// WalletTransactionsGET contains the specified set of confirmed and
	// unconfirmed transactions.
	WalletTransactionsGET struct {
		ConfirmedTransactions   []modules.ProcessedTransaction `json:"confirmedtransactions"`
		UnconfirmedTransactions []modules.ProcessedTransaction `json:"unconfirmedtransactions"`
	}

	// WalletTransactionsGETaddr contains the set of wallet transactions
	// relevant to the input address provided in the call to
	// /wallet/transaction/:addr
	WalletTransactionsGETaddr struct {
		ConfirmedTransactions   []modules.ProcessedTransaction `json:"confirmedtransactions"`
		UnconfirmedTransactions []modules.ProcessedTransaction `json:"unconfirmedtransactions"`
	}

	// WalletUnlockConditionsGET contains a set of unlock conditions.
	WalletUnlockConditionsGET struct {
		UnlockConditions types.UnlockConditions `json:"unlockconditions"`
	}

	// WalletUnlockConditionsPOSTParams contains a set of unlock conditions.
	WalletUnlockConditionsPOSTParams struct {
		UnlockConditions types.UnlockConditions `json:"unlockconditions"`
	}

	// WalletUnspentGET contains the unspent outputs tracked by the wallet.
	// The MaturityHeight field of each output indicates the height of the
	// block that the output appeared in.
	WalletUnspentGET struct {
		Outputs []modules.UnspentOutput `json:"outputs"`
	}

	// WalletVerifyAddressGET contains a bool indicating if the address passed to
	// /wallet/verify/address/:addr is a valid address.
	WalletVerifyAddressGET struct {
		Valid bool `json:"valid"`
	}

	// WalletVerifyPasswordGET contains a bool indicating if the password passed
	// to /wallet/verifypassword is the password being used to encrypt the
	// wallet.
	WalletVerifyPasswordGET struct {
		Valid bool `json:"valid"`
	}

	// WalletWatchPOST contains the set of addresses to add or remove from the
	// watch set.
	WalletWatchPOST struct {
		Addresses []types.UnlockHash `json:"addresses"`
		Remove    bool               `json:"remove"`
		Unused    bool               `json:"unused"`
	}

	// WalletWatchGET contains the set of addresses that the wallet is
	// currently watching.
	WalletWatchGET struct {
		Addresses []types.UnlockHash `json:"addresses"`
	}
)

// RegisterRoutesWallet is a helper function to register all wallet routes.
func RegisterRoutesWallet(router *httprouter.Router, wallet modules.Wallet, requiredPassword string) {
	router.GET("/wallet", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletHandler(wallet, w, req, ps)
	})
	router.POST("/wallet/033x", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		wallet033xHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/address", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletAddressHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/addresses", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletAddressesHandler(wallet, w, req, ps)
	})
	router.GET("/wallet/seedaddrs", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSeedAddressesHandler(wallet, w, req, ps)
	})
	router.GET("/wallet/backup", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletBackupHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/init", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletInitHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/init/seed", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletInitSeedHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/lock", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletLockHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/seed", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSeedHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/seeds", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSeedsHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/siacoins", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSiacoinsHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/siafunds", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSiafundsHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/siagkey", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSiagkeyHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/sweep/seed", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSweepSeedHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/transaction/:id", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletTransactionHandler(wallet, w, req, ps)
	})
	router.GET("/wallet/transactions", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletTransactionsHandler(wallet, w, req, ps)
	})
	router.GET("/wallet/transactions/:addr", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletTransactionsAddrHandler(wallet, w, req, ps)
	})
	router.GET("/wallet/verify/address/:addr", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletVerifyAddressHandler(w, req, ps)
	})
	router.POST("/wallet/unlock", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletUnlockHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/changepassword", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletChangePasswordHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/verifypassword", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletVerifyPasswordHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/unlockconditions/:addr", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletUnlockConditionsHandlerGET(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/unlockconditions", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletUnlockConditionsHandlerPOST(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/unspent", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletUnspentHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/sign", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletSignHandler(wallet, w, req, ps)
	}, requiredPassword))
	router.GET("/wallet/watch", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletWatchHandlerGET(wallet, w, req, ps)
	}, requiredPassword))
	router.POST("/wallet/watch", RequirePassword(func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		walletWatchHandlerPOST(wallet, w, req, ps)
	}, requiredPassword))
}

// encryptionKeys enumerates the possible encryption keys that can be derived
// from an input string.
func encryptionKeys(seedStr string) (validKeys []crypto.CipherKey, seeds []modules.Seed) {
	dicts := []mnemonics.DictionaryID{"english", "german", "japanese"}
	for _, dict := range dicts {
		seed, err := modules.StringToSeed(seedStr, dict)
		if err != nil {
			continue
		}
		validKeys = append(validKeys, crypto.NewWalletKey(crypto.HashObject(seed)))
		seeds = append(seeds, seed)
	}
	validKeys = append(validKeys, crypto.NewWalletKey(crypto.HashObject(seedStr)))
	return
}

// walletHander handles API calls to /wallet.
func walletHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	siacoinBal, siafundBal, siaclaimBal, err := wallet.ConfirmedBalance()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	siacoinsOut, siacoinsIn, err := wallet.UnconfirmedBalance()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	dustThreshold, err := wallet.DustThreshold()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	encrypted, err := wallet.Encrypted()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	unlocked, err := wallet.Unlocked()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	rescanning, err := wallet.Rescanning()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	height, err := wallet.Height()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet: %v", err)}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletGET{
		Encrypted:  encrypted,
		Unlocked:   unlocked,
		Rescanning: rescanning,
		Height:     height,

		ConfirmedSiacoinBalance:     siacoinBal,
		UnconfirmedOutgoingSiacoins: siacoinsOut,
		UnconfirmedIncomingSiacoins: siacoinsIn,

		SiafundBalance:      siafundBal,
		SiacoinClaimBalance: siaclaimBal,

		DustThreshold: dustThreshold,
	})
}

// wallet033xHandler handles API calls to /wallet/033x.
func wallet033xHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	source := req.FormValue("source")
	// Check that source is an absolute paths.
	if !filepath.IsAbs(source) {
		WriteError(w, Error{"error when calling /wallet/033x: source must be an absolute path"}, http.StatusBadRequest)
		return
	}
	potentialKeys, _ := encryptionKeys(req.FormValue("encryptionpassword"))
	for _, key := range potentialKeys {
		err := wallet.Load033xWallet(key, source)
		if err == nil {
			WriteSuccess(w)
			return
		}
		if !errors.Contains(err, modules.ErrBadEncryptionKey) {
			WriteError(w, Error{"error when calling /wallet/033x: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}
	WriteError(w, Error{modules.ErrBadEncryptionKey.Error()}, http.StatusBadRequest)
}

// walletAddressHandler handles API calls to /wallet/address.
func walletAddressHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	unlockConditions, err := wallet.NextAddress()
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/addresses: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletAddressGET{
		Address: unlockConditions.UnlockHash(),
	})
}

// walletSeedAddressesHandler handles the requests to /wallet/seedaddrs.
func walletSeedAddressesHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// Parse the count argument. If it isn't specified we return as many
	// addresses as possible.
	count := uint64(math.MaxUint64)
	c := req.FormValue("count")
	if c != "" {
		_, err := fmt.Sscan(c, &count)
		if err != nil {
			WriteError(w, Error{"Failed to parse count: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}
	// Get the last count addresses.
	addresses, err := wallet.LastAddresses(count)
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet/addresses: %v", err)}, http.StatusBadRequest)
		return
	}
	// Send the response.
	WriteJSON(w, WalletAddressesGET{
		Addresses: addresses,
	})
}

// walletAddressHandler handles API calls to /wallet/addresses.
func walletAddressesHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	addresses, err := wallet.AllAddresses()
	if err != nil {
		WriteError(w, Error{fmt.Sprintf("Error when calling /wallet/addresses: %v", err)}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletAddressesGET{
		Addresses: addresses,
	})
}

// walletBackupHandler handles API calls to /wallet/backup.
func walletBackupHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	destination := req.FormValue("destination")
	// Check that the destination is absolute.
	if !filepath.IsAbs(destination) {
		WriteError(w, Error{"error when calling /wallet/backup: destination must be an absolute path"}, http.StatusBadRequest)
		return
	}
	err := wallet.CreateBackup(destination)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/backup: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}

// walletInitHandler handles API calls to /wallet/init.
func walletInitHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var encryptionKey crypto.CipherKey
	if req.FormValue("encryptionpassword") != "" {
		encryptionKey = crypto.NewWalletKey(crypto.HashObject(req.FormValue("encryptionpassword")))
	}

	if req.FormValue("force") == "true" {
		err := wallet.Reset()
		if err != nil {
			WriteError(w, Error{"error when calling /wallet/init: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}
	seed, err := wallet.Encrypt(encryptionKey)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/init: " + err.Error()}, http.StatusBadRequest)
		return
	}

	dictID := mnemonics.DictionaryID(req.FormValue("dictionary"))
	if dictID == "" {
		dictID = "english"
	}
	seedStr, err := modules.SeedToString(seed, dictID)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/init: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletInitPOST{
		PrimarySeed: seedStr,
	})
}

// walletInitSeedHandler handles API calls to /wallet/init/seed.
func walletInitSeedHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var encryptionKey crypto.CipherKey
	if req.FormValue("encryptionpassword") != "" {
		encryptionKey = crypto.NewWalletKey(crypto.HashObject(req.FormValue("encryptionpassword")))
	}
	dictID := mnemonics.DictionaryID(req.FormValue("dictionary"))
	if dictID == "" {
		dictID = "english"
	}
	seed, err := modules.StringToSeed(req.FormValue("seed"), dictID)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/init/seed: " + err.Error()}, http.StatusBadRequest)
		return
	}

	if req.FormValue("force") == "true" {
		err = wallet.Reset()
		if err != nil {
			WriteError(w, Error{"error when calling /wallet/init/seed: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}

	err = wallet.InitFromSeed(encryptionKey, seed)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/init/seed: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}

// walletSeedHandler handles API calls to /wallet/seed.
func walletSeedHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// Get the seed using the dictionary + phrase
	dictID := mnemonics.DictionaryID(req.FormValue("dictionary"))
	if dictID == "" {
		dictID = "english"
	}
	seed, err := modules.StringToSeed(req.FormValue("seed"), dictID)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/seed: " + err.Error()}, http.StatusBadRequest)
		return
	}

	potentialKeys, _ := encryptionKeys(req.FormValue("encryptionpassword"))
	for _, key := range potentialKeys {
		err := wallet.LoadSeed(key, seed)
		if err == nil {
			WriteSuccess(w)
			return
		}
		if !errors.Contains(err, modules.ErrBadEncryptionKey) {
			WriteError(w, Error{"error when calling /wallet/seed: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}
	WriteError(w, Error{"error when calling /wallet/seed: " + modules.ErrBadEncryptionKey.Error()}, http.StatusBadRequest)
}

// walletSiagkeyHandler handles API calls to /wallet/siagkey.
func walletSiagkeyHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// Fetch the list of keyfiles from the post body.
	keyfiles := strings.Split(req.FormValue("keyfiles"), ",")
	potentialKeys, _ := encryptionKeys(req.FormValue("encryptionpassword"))

	for _, keypath := range keyfiles {
		// Check that all key paths are absolute paths.
		if !filepath.IsAbs(keypath) {
			WriteError(w, Error{"error when calling /wallet/siagkey: keyfiles contains a non-absolute path"}, http.StatusBadRequest)
			return
		}
	}

	for _, key := range potentialKeys {
		err := wallet.LoadSiagKeys(key, keyfiles)
		if err == nil {
			WriteSuccess(w)
			return
		}
		if !errors.Contains(err, modules.ErrBadEncryptionKey) {
			WriteError(w, Error{"error when calling /wallet/siagkey: " + err.Error()}, http.StatusBadRequest)
			return
		}
	}
	WriteError(w, Error{"error when calling /wallet/siagkey: " + modules.ErrBadEncryptionKey.Error()}, http.StatusBadRequest)
}

// walletLockHandler handles API calls to /wallet/lock.
func walletLockHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	err := wallet.Lock()
	if err != nil {
		WriteError(w, Error{err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}

// walletSeedsHandler handles API calls to /wallet/seeds.
func walletSeedsHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	dictionary := mnemonics.DictionaryID(req.FormValue("dictionary"))
	if dictionary == "" {
		dictionary = mnemonics.English
	}

	// Get the primary seed information.
	primarySeed, addrsRemaining, err := wallet.PrimarySeed()
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/seeds: " + err.Error()}, http.StatusBadRequest)
		return
	}
	primarySeedStr, err := modules.SeedToString(primarySeed, dictionary)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/seeds: " + err.Error()}, http.StatusBadRequest)
		return
	}

	// Get the list of seeds known to the wallet.
	allSeeds, err := wallet.AllSeeds()
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/seeds: " + err.Error()}, http.StatusBadRequest)
		return
	}
	var allSeedsStrs []string
	for _, seed := range allSeeds {
		str, err := modules.SeedToString(seed, dictionary)
		if err != nil {
			WriteError(w, Error{"error when calling /wallet/seeds: " + err.Error()}, http.StatusBadRequest)
			return
		}
		allSeedsStrs = append(allSeedsStrs, str)
	}
	WriteJSON(w, WalletSeedsGET{
		PrimarySeed:        primarySeedStr,
		AddressesRemaining: int(addrsRemaining),
		AllSeeds:           allSeedsStrs,
	})
}

// walletSiacoinsHandler handles API calls to /wallet/siacoins.
func walletSiacoinsHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var txns []types.Transaction
	if req.FormValue("outputs") != "" {
		// multiple amounts + destinations
		if req.FormValue("amount") != "" || req.FormValue("destination") != "" || req.FormValue("feeIncluded") != "" {
			WriteError(w, Error{"cannot supply both 'outputs' and single amount+destination pair and/or feeIncluded parameter"}, http.StatusInternalServerError)
			return
		}

		var outputs []types.SiacoinOutput
		err := json.Unmarshal([]byte(req.FormValue("outputs")), &outputs)
		if err != nil {
			WriteError(w, Error{"could not decode outputs: " + err.Error()}, http.StatusInternalServerError)
			return
		}
		txns, err = wallet.SendSiacoinsMulti(outputs)
		if err != nil {
			WriteError(w, Error{"error when calling /wallet/siacoins: " + err.Error()}, http.StatusInternalServerError)
			return
		}
	} else {
		// single amount + destination
		amount, ok := scanAmount(req.FormValue("amount"))
		if !ok {
			WriteError(w, Error{"could not read amount from POST call to /wallet/siacoins"}, http.StatusBadRequest)
			return
		}
		dest, err := scanAddress(req.FormValue("destination"))
		if err != nil {
			WriteError(w, Error{"could not read address from POST call to /wallet/siacoins"}, http.StatusBadRequest)
			return
		}
		feeIncluded, err := scanBool(req.FormValue("feeIncluded"))
		if err != nil {
			WriteError(w, Error{"could not read feeIncluded from POST call to /wallet/siacoins"}, http.StatusBadRequest)
			return
		}

		if feeIncluded {
			txns, err = wallet.SendSiacoinsFeeIncluded(amount, dest)
		} else {
			txns, err = wallet.SendSiacoins(amount, dest)
		}
		if err != nil {
			WriteError(w, Error{"error when calling /wallet/siacoins: " + err.Error()}, http.StatusInternalServerError)
			return
		}
	}

	var txids []types.TransactionID
	for _, txn := range txns {
		txids = append(txids, txn.ID())
	}
	WriteJSON(w, WalletSiacoinsPOST{
		Transactions:   txns,
		TransactionIDs: txids,
	})
}

// walletSiafundsHandler handles API calls to /wallet/siafunds.
func walletSiafundsHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	amount, ok := scanAmount(req.FormValue("amount"))
	if !ok {
		WriteError(w, Error{"could not read 'amount' from POST call to /wallet/siafunds"}, http.StatusBadRequest)
		return
	}
	dest, err := scanAddress(req.FormValue("destination"))
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/siafunds: " + err.Error()}, http.StatusBadRequest)
		return
	}

	txns, err := wallet.SendSiafunds(amount, dest)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/siafunds: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	var txids []types.TransactionID
	for _, txn := range txns {
		txids = append(txids, txn.ID())
	}
	WriteJSON(w, WalletSiafundsPOST{
		Transactions:   txns,
		TransactionIDs: txids,
	})
}

// walletSweepSeedHandler handles API calls to /wallet/sweep/seed.
func walletSweepSeedHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// Get the seed using the dictionary + phrase
	dictID := mnemonics.DictionaryID(req.FormValue("dictionary"))
	if dictID == "" {
		dictID = "english"
	}
	seed, err := modules.StringToSeed(req.FormValue("seed"), dictID)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/sweep/seed: " + err.Error()}, http.StatusBadRequest)
		return
	}

	coins, funds, err := wallet.SweepSeed(seed)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/sweep/seed: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletSweepPOST{
		Coins: coins,
		Funds: funds,
	})
}

// walletTransactionHandler handles API calls to /wallet/transaction/:id.
func walletTransactionHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
	// Parse the id from the url.
	var id types.TransactionID
	jsonID := "\"" + ps.ByName("id") + "\""
	err := id.UnmarshalJSON([]byte(jsonID))
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transaction/id: " + err.Error()}, http.StatusBadRequest)
		return
	}

	txn, ok, err := wallet.Transaction(id)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transaction/id: " + err.Error()}, http.StatusBadRequest)
		return
	}
	if !ok {
		WriteError(w, Error{"error when calling /wallet/transaction/id  :  transaction not found"}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletTransactionGETid{
		Transaction: txn,
	})
}

// walletTransactionsHandler handles API calls to /wallet/transactions.
func walletTransactionsHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	startheightStr, endheightStr := req.FormValue("startheight"), req.FormValue("endheight")
	if startheightStr == "" || endheightStr == "" {
		WriteError(w, Error{"startheight and endheight must be provided to a /wallet/transactions call."}, http.StatusBadRequest)
		return
	}
	// Get the start and end blocks.
	start, err := strconv.ParseUint(startheightStr, 10, 64)
	if err != nil {
		WriteError(w, Error{"parsing integer value for parameter `startheight` failed: " + err.Error()}, http.StatusBadRequest)
		return
	}
	// Check if endheightStr is set to -1. If it is, we use MaxUint64 as the
	// end. Otherwise we parse the argument as an unsigned integer.
	var end uint64
	if endheightStr == "-1" {
		end = math.MaxUint64
	} else {
		end, err = strconv.ParseUint(endheightStr, 10, 64)
	}
	if err != nil {
		WriteError(w, Error{"parsing integer value for parameter `endheight` failed: " + err.Error()}, http.StatusBadRequest)
		return
	}
	confirmedTxns, err := wallet.Transactions(types.BlockHeight(start), types.BlockHeight(end))
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transactions: " + err.Error()}, http.StatusBadRequest)
		return
	}
	unconfirmedTxns, err := wallet.UnconfirmedTransactions()
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transactions: " + err.Error()}, http.StatusBadRequest)
		return
	}

	WriteJSON(w, WalletTransactionsGET{
		ConfirmedTransactions:   confirmedTxns,
		UnconfirmedTransactions: unconfirmedTxns,
	})
}

// walletTransactionsAddrHandler handles API calls to
// /wallet/transactions/:addr.
func walletTransactionsAddrHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
	// Parse the address being input.
	jsonAddr := "\"" + ps.ByName("addr") + "\""
	var addr types.UnlockHash
	err := addr.UnmarshalJSON([]byte(jsonAddr))
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transactions: " + err.Error()}, http.StatusBadRequest)
		return
	}

	confirmedATs, err := wallet.AddressTransactions(addr)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transactions: " + err.Error()}, http.StatusBadRequest)
		return
	}
	unconfirmedATs, err := wallet.AddressUnconfirmedTransactions(addr)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/transactions: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletTransactionsGETaddr{
		ConfirmedTransactions:   confirmedATs,
		UnconfirmedTransactions: unconfirmedATs,
	})
}

// walletUnlockHandler handles API calls to /wallet/unlock.
func walletUnlockHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	potentialKeys, _ := encryptionKeys(req.FormValue("encryptionpassword"))
	var err error
	for _, key := range potentialKeys {
		errChan := wallet.UnlockAsync(key)
		var unlockErr error
		select {
		case unlockErr = <-errChan:
		default:
		}
		if unlockErr == nil {
			WriteSuccess(w)
			return
		}
		err = errors.Compose(err, unlockErr)
	}
	WriteError(w, Error{"error when calling /wallet/unlock: " + err.Error()}, http.StatusBadRequest)
}

// walletChangePasswordHandler handles API calls to /wallet/changepassword
func walletChangePasswordHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var newKey crypto.CipherKey
	newPassword := req.FormValue("newpassword")
	if newPassword == "" {
		WriteError(w, Error{"a password must be provided to newpassword"}, http.StatusBadRequest)
		return
	}
	newKey = crypto.NewWalletKey(crypto.HashObject(newPassword))

	originalKeys, seeds := encryptionKeys(req.FormValue("encryptionpassword"))
	var err error
	for _, key := range originalKeys {
		keyErr := wallet.ChangeKey(key, newKey)
		if keyErr == nil {
			WriteSuccess(w)
			return
		}
		err = errors.Compose(err, keyErr)
	}
	for _, seed := range seeds {
		seedErr := wallet.ChangeKeyWithSeed(seed, newKey)
		if seedErr == nil {
			WriteSuccess(w)
			return
		}
		err = errors.Compose(err, seedErr)
	}
	WriteError(w, Error{"error when calling /wallet/changepassword: " + err.Error()}, http.StatusBadRequest)
	return
}

// walletVerifyPasswordHandler handles API calls to /wallet/verifypassword
func walletVerifyPasswordHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	originalKeys, _ := encryptionKeys(req.FormValue("password"))
	var err error
	for _, key := range originalKeys {
		valid, keyErr := wallet.IsMasterKey(key)
		if keyErr == nil {
			WriteJSON(w, WalletVerifyPasswordGET{
				Valid: valid,
			})
			return
		}
		err = errors.Compose(err, keyErr)
	}
	WriteError(w, Error{"error when calling /wallet/verifypassword: " + err.Error()}, http.StatusBadRequest)
}

// walletVerifyAddressHandler handles API calls to /wallet/verify/address/:addr.
func walletVerifyAddressHandler(w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
	addrString := ps.ByName("addr")

	err := new(types.UnlockHash).LoadString(addrString)
	WriteJSON(w, WalletVerifyAddressGET{Valid: err == nil})
}

// walletUnlockConditionsHandlerGET handles GET calls to /wallet/unlockconditions.
func walletUnlockConditionsHandlerGET(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
	var addr types.UnlockHash
	err := addr.LoadString(ps.ByName("addr"))
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/unlockconditions: " + err.Error()}, http.StatusBadRequest)
		return
	}
	uc, err := wallet.UnlockConditions(addr)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/unlockconditions: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletUnlockConditionsGET{
		UnlockConditions: uc,
	})
}

// walletUnlockConditionsHandlerPOST handles POST calls to /wallet/unlockconditions.
func walletUnlockConditionsHandlerPOST(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var params WalletUnlockConditionsPOSTParams
	err := json.NewDecoder(req.Body).Decode(&params)
	if err != nil {
		WriteError(w, Error{"invalid parameters: " + err.Error()}, http.StatusBadRequest)
		return
	}
	err = wallet.AddUnlockConditions(params.UnlockConditions)
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/unlockconditions: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}

// walletUnspentHandler handles API calls to /wallet/unspent.
func walletUnspentHandler(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	outputs, err := wallet.UnspentOutputs()
	if err != nil {
		WriteError(w, Error{"error when calling /wallet/unspent: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	WriteJSON(w, WalletUnspentGET{
		Outputs: outputs,
	})
}

// walletSignHandler handles API calls to /wallet/sign.
func walletSignHandler(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var params WalletSignPOSTParams
	err := json.NewDecoder(req.Body).Decode(&params)
	if err != nil {
		WriteError(w, Error{"invalid parameters: " + err.Error()}, http.StatusBadRequest)
		return
	}
	err = wallet.SignTransaction(&params.Transaction, params.ToSign)
	if err != nil {
		WriteError(w, Error{"failed to sign transaction: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletSignPOSTResp{
		Transaction: params.Transaction,
	})
}

// walletWatchHandlerGET handles GET calls to /wallet/watch.
func walletWatchHandlerGET(wallet modules.Wallet, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	addrs, err := wallet.WatchAddresses()
	if err != nil {
		WriteError(w, Error{"failed to get watch addresses: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteJSON(w, WalletWatchGET{
		Addresses: addrs,
	})
}

// walletWatchHandlerPOST handles POST calls to /wallet/watch.
func walletWatchHandlerPOST(wallet modules.Wallet, w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var wwpp WalletWatchPOST
	err := json.NewDecoder(req.Body).Decode(&wwpp)
	if err != nil {
		WriteError(w, Error{"invalid parameters: " + err.Error()}, http.StatusBadRequest)
		return
	}
	if wwpp.Remove {
		err = wallet.RemoveWatchAddresses(wwpp.Addresses, wwpp.Unused)
	} else {
		err = wallet.AddWatchAddresses(wwpp.Addresses, wwpp.Unused)
	}
	if err != nil {
		WriteError(w, Error{"failed to update watch set: " + err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}
