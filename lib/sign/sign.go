package sign

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/cognitoidentity"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
)

type AwsSigner struct {
	Service        string
	Region         string
	Session        *session.Session
	IdentityPoolId string
	Issuer         string
	Token          string
	IgnoreFailure  bool
}

type UserPasswordAwsSigner struct {
	Service            string
	Region             string
	Session            *session.Session
	IdentityPoolId     string
	Issuer             string
	IgnoreFailure      bool
	Username           string
	Password           string
	CognitoAppClientId string
}

func (s AwsSigner) SignRequest(req *http.Request) error {

	svc := cognitoidentity.New(s.Session, aws.NewConfig().WithRegion("us-east-1")/*.WithLogLevel(aws.LogDebug)*/) 

	/*log.Printf("DEBUG: Logins map being sent to Cognito: %+v", map[string]*string{
		s.Issuer: aws.String(s.Token),
	})*/

	// Call decodeIssuerFromToken and handle both `issuer` and `error`
	/*tokenIssuer, err := decodeIssuerFromToken(s.Token)
	if err != nil {
		log.Printf("Error decoding issuer from token: %v", err)
		return err // or handle the error appropriately
	}

	// Now safely use tokenIssuer
	log.Printf("Decoded Token issuer: %s", tokenIssuer)

	if s.Issuer != tokenIssuer {
		log.Printf("ERROR: Issuer mismatch! Expected: %s, Got: %s", s.Issuer, tokenIssuer)
	} else {
		log.Printf("INFO: Issuer match! Expected: %s, Got: %s", s.Issuer, tokenIssuer)
	}*/
	
	// Call decodeJWTBody and handle both returned values
	/*decodedPayload, err := decodeJWTBody(s.Token)
	if err != nil {
		log.Printf("Error decoding JWT body: %v", err)
		return fmt.Errorf("Unable to decode body") // Handle the error appropriately (e.g., return or log)
	}
	
	if decodedPayload["aud"] == "7h778muira8dkr69dt35jhbjo8" {
		log.Printf("INFO: Token `aud` match. Expected: %s, Got: %s", "7h778muira8dkr69dt35jhbjo8", decodedPayload["aud"])
	} else {
		log.Printf("ERROR: Token `aud` mismatch. Expected: %s, Got: %s", "7h778muira8dkr69dt35jhbjo8", decodedPayload["aud"])
	}

	// Check if the token is expired
	expired, err := isTokenExpired(decodedPayload)
	if err != nil {
		log.Printf("Error checking token expiration: %v", err)
		return fmt.Errorf("Error checking token expiration.")
	}

	if expired {
		log.Printf("The token has expired.")
	} else {
		log.Printf("The token is still valid.")
	}*/

	idRes, err := svc.GetId(&cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(s.IdentityPoolId),
		Logins: map[string]*string{
			s.Issuer: aws.String(s.Token),
		},
	})

	// log.Printf("The issuer is: %s", s.Issuer)

	if err != nil {
		log.Print("SignRequest GetId() error (signer)", err.Error())
		// @todo: For now since indexing isn't required at the moment.
		return nil
	}

	credRes, err := svc.GetCredentialsForIdentity(&cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: idRes.IdentityId,
		Logins: map[string]*string{
			s.Issuer: aws.String(s.Token),
		},
	})
	if err != nil {
		log.Print("SignRequest GetId() 2 error (signer)", err.Error())
		// @todo: For now since indexing isn't required at the moment.
		return nil
	}

	// credentials := credentials.NewEnvCredentials()

	credentials := credentials.NewStaticCredentials(
		*credRes.Credentials.AccessKeyId,
		*credRes.Credentials.SecretKey,
		*credRes.Credentials.SessionToken,
	)

	signer := v4.NewSigner(credentials)
	var b []byte
	if req.Body == nil {
		b = make([]byte, 0)
	} else {
		b2, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		} else {
			b = b2
		}
	}
	body := bytes.NewReader(b)
	/*hash := sha256.New()
	var hb []byte
	hash.Write(hb)
	req.Header.Add("X-Amz-Content-Sha256", string(hb))*/
	_, err = signer.Sign(req, body, s.Service, s.Region, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (s UserPasswordAwsSigner) SignRequest(req *http.Request) error {

	svc := cognitoidentity.New(s.Session, aws.NewConfig().WithRegion("us-east-1"))

	authParams := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: aws.String("USER_PASSWORD_AUTH"),
		AuthParameters: map[string]*string{
			"USERNAME": aws.String(s.Username),
			"PASSWORD": aws.String(s.Password),
		},
		ClientId: aws.String(s.CognitoAppClientId), // this is the app client ID
	}
	cip := cognitoidentityprovider.New(s.Session, aws.NewConfig().WithRegion("us-east-1"))
	authResp, err := cip.InitiateAuth(authParams)
	if err != nil {
		log.Print("InitiateAuth() error", err.Error())
		// @todo: For now since indexing isn't required at the moment.
		return nil
	}

	idRes, err := svc.GetId(&cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(s.IdentityPoolId),
		Logins: map[string]*string{
			s.Issuer: authResp.AuthenticationResult.IdToken,
		},
	})
	if err != nil {
		log.Print("SignRequest GetId() error (password)", err.Error())
		// @todo: For now since indexing isn't required at the moment.
		return nil
	}

	credRes, err := svc.GetCredentialsForIdentity(&cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: idRes.IdentityId,
		Logins: map[string]*string{
			s.Issuer: authResp.AuthenticationResult.IdToken,
		},
	})
	if err != nil {
		log.Print("SignRequest GetId() 2 error (password)", err.Error())
		// @todo: For now since indexing isn't required at the moment.
		return nil
	}

	// credentials := credentials.NewEnvCredentials()

	credentials := credentials.NewStaticCredentials(
		*credRes.Credentials.AccessKeyId,
		*credRes.Credentials.SecretKey,
		*credRes.Credentials.SessionToken,
	)

	signer := v4.NewSigner(credentials)
	var b []byte
	if req.Body == nil {
		b = make([]byte, 0)
	} else {
		b2, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		} else {
			b = b2
		}
	}
	body := bytes.NewReader(b)
	/*hash := sha256.New()
	var hb []byte
	hash.Write(hb)
	req.Header.Add("X-Amz-Content-Sha256", string(hb))*/
	_, err = signer.Sign(req, body, s.Service, s.Region, time.Now())
	if err != nil {
		return err
	}
	return nil
}

// decodeIssuerFromToken extracts and returns the `iss` (issuer) claim from a JWT token.
func decodeIssuerFromToken(token string) (string, error) {
	// Split the token into its three components: header, payload, and signature.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format: expected 3 parts but got %d", len(parts))
	}

	// Decode the payload (second part of the token).
	decodedPayload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode token payload: %w", err)
	}

	// Parse the payload as JSON to extract the `iss` field.
	var claims map[string]interface{}
	if err := json.Unmarshal(decodedPayload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse token payload as JSON: %w", err)
	}

	// Extract the `iss` claim.
	issuer, ok := claims["iss"].(string)
	if !ok {
		return "", fmt.Errorf("issuer (`iss`) not found or is not a string in token payload")
	}

	return issuer, nil
}

// decodeJWTBody extracts the payload of a JWT token and returns it as a map
func decodeJWTBody(token string) (map[string]interface{}, error) {
	// Split the token into its three components: header, payload, and signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format: expected 3 parts but got %d", len(parts))
	}

	// Decode the payload (second part of the token) using Base64 URL decoding
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to Base64 decode JWT payload: %w", err)
	}

	// Unmarshal the JSON payload into a map[string]interface{}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT payload as JSON: %w", err)
	}

	return claims, nil
}

// isTokenExpired checks the expiration (`exp`) claim against the current time
func isTokenExpired(claims map[string]interface{}) (bool, error) {
	// Extract the `exp` claim from the claims map
	exp, ok := claims["exp"].(float64) // JWT stores timestamps as numbers
	if !ok {
		return false, fmt.Errorf("expiration (`exp`) claim missing or not a valid number")
	}

	// Compare the `exp` timestamp with the current time
	currentTime := time.Now().UTC().Unix()
	expTime := int64(exp) // Convert float64 to int64 for timestamp comparison

	// Return true if token is expired
	return expTime < currentTime, nil
}