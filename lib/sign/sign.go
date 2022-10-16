package sign

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/cognitoidentity"
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

func (s AwsSigner) SignRequest(req *http.Request) error {

	svc := cognitoidentity.New(s.Session)

	idRes, err := svc.GetId(&cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(s.IdentityPoolId),
		Logins: map[string]*string{
			s.Issuer: aws.String(s.Token),
		},
	})
	if err != nil {
		log.Print("SignRequest GetId() error", err.Error())
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
		log.Print("SignRequest GetId() error", err.Error())
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
