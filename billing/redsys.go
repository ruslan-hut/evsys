package billing

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"evsys/internal"
	"evsys/models"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// for tests: https://sis-t.redsys.es:25443/sis/rest/trataPeticionREST
	// production: https://sis.redsys.es/sis/rest/trataPeticionREST
	apiUrl = "https://sis-t.redsys.es:25443/sis/rest/trataPeticionREST"
)

type RedSys struct {
	database internal.Database
	logger   internal.LogHandler
}

func NewRedSys() *RedSys {
	return &RedSys{}
}

func (r *RedSys) SetDatabase(database internal.Database) {
	r.database = database
}

func (r *RedSys) SetLogger(logger internal.LogHandler) {
	r.logger = logger
}

func (r *RedSys) TransactionPayment(transaction *models.Transaction) {
	amount := transaction.PaymentAmount - transaction.PaymentBilled
	if amount <= 0 || transaction.IsFinished || transaction.Username == "" {
		return
	}

	tag, err := r.database.GetUserTag(transaction.IdTag)
	if err != nil {
		r.logger.Error("payment: failed to get user tag: %s", err)
		return
	}
	paymentMethod, err := r.database.GetPaymentMethod(tag.UserId)
	if err != nil {
		r.logger.Error("payment: failed to get payment method: %s", err)
		return
	}
	consumed := (transaction.MeterStop - transaction.MeterStart) / 1000
	description := fmt.Sprintf("%s:%d %dkW", transaction.ChargePointId, transaction.ConnectorId, consumed)

	orderToClose, _ := r.database.GetPaymentOrderByTransaction(transaction.Id)
	if orderToClose != nil {
		orderToClose.IsCompleted = true
		orderToClose.Result = "closed without response"
		orderToClose.TimeClosed = time.Now()
		_ = r.database.SavePaymentOrder(orderToClose)
	}

	paymentOrder := models.PaymentOrder{
		Amount:        amount,
		Description:   description,
		Identifier:    paymentMethod.Identifier,
		TransactionId: transaction.Id,
		UserId:        tag.UserId,
		UserName:      tag.Username,
		TimeOpened:    time.Now(),
	}

	lastOrder, _ := r.database.GetLastOrder()
	if lastOrder != nil {
		paymentOrder.Order = lastOrder.Order + 1
	} else {
		paymentOrder.Order = 1200
	}

	err = r.database.SavePaymentOrder(&paymentOrder)
	if err != nil {
		r.logger.Error("payment: failed to save paymentOrder: %s", err)
		return
	}

	order := fmt.Sprintf("%d", paymentOrder.Order)
	secret := "sq7HjrUOBfKmC576ILgskD5srU870gJ7"

	parameters := models.MerchantParameters{
		Amount:          fmt.Sprintf("%d", amount),
		Order:           order,
		Identifier:      paymentOrder.Identifier,
		MerchantCode:    "358333276",
		Currency:        "978",
		TransactionType: "0",
		Terminal:        "001",
		DirectPayment:   "true",
		Exception:       "MIT",
		Cof:             "N",
	}

	// encode parameters to Base64
	parametersBase64, err := createParameters(&parameters)
	if err != nil {
		r.logger.Error("payment: failed to create parameters: %s", err)
		return
	}
	signature, err := createSignature(order, parametersBase64, secret)
	if err != nil {
		r.logger.Error("payment: failed to create signature: %s", err)
		return
	}

	request := models.PaymentRequest{
		Parameters:       parametersBase64,
		Signature:        signature,
		SignatureVersion: "HMAC_SHA256_V1",
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		r.logger.Error("payment: failed to create request: %s", err)
		return
	}

	response, err := http.Post(apiUrl, "application/json", bytes.NewBuffer(requestData))
	if err != nil {
		r.logger.Error("payment: failed to send request: %s", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			r.logger.Error("payment: failed to close response body: %s", err)
		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		r.logger.Error("payment: failed to read response body: %s", err)
		return
	}

	var paymentResponse models.PaymentRequest
	err = json.Unmarshal(body, &paymentResponse)
	if err != nil {
		r.logger.Warn(fmt.Sprintf("payment: response: %s", string(body)))
		r.logger.Error("payment: failed to parse response: %s", err)
		return
	}
}

func createParameters(parameters *models.MerchantParameters) (string, error) {
	// convert parameters to JSON string
	parametersJson, err := json.Marshal(parameters)
	if err != nil {
		return "", err
	}
	// encode parameters to Base64
	return base64.StdEncoding.EncodeToString(parametersJson), nil
}

func createSignature(order, parameters, secret string) (string, error) {
	// decode signature from Base64
	signatureBase64, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}
	// convert signature to Hexadecimal
	signatureHex := hex.EncodeToString(signatureBase64)
	// encrypt signature with 3DES
	signatureEncrypted, err := encrypt3DES(signatureHex, order)
	if err != nil {
		return "", err
	}
	// create hash with SHA256
	hash := mac256(parameters, signatureEncrypted)
	// encode hash to Base64
	return base64.StdEncoding.EncodeToString(hash), nil
}

func mac256(message string, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func encrypt3DES(secretKc, order string) ([]byte, error) {
	// Convert secretKc and order to byte arrays
	key, err := hex.DecodeString(secretKc)
	if err != nil {
		return nil, err
	}

	iv := []byte(order) // Use order as IV

	// Create a new 3DES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Encrypt with 3DES
	ciphertext := make([]byte, len(iv))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, iv)

	return ciphertext, nil
}
