package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/x/mongo/driver/auth"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/trufflesecurity/trufflehog/v3/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/detectorspb"
)

type Scanner struct {
	timeout time.Duration // Zero value means "default timeout"
}

// Ensure the Scanner satisfies the interface at compile time.
var _ detectors.Detector = (*Scanner)(nil)

var (
	defaultTimeout = 2 * time.Second
	// Make sure that your group is surrounded in boundary characters such as below to reduce false positives.
	keyPat = regexp.MustCompile(`\b(mongodb(\+srv)?://[\S]{3,50}:([\S]{3,88})@[-.%\w\/:]+)\b`)
	// TODO: Add support for sharded cluster, replica set and Atlas Deployment.
)

// Keywords are used for efficiently pre-filtering chunks.
// Use identifiers in the secret preferably, or the provider name.
func (s Scanner) Keywords() []string {
	return []string{"mongodb"}
}

// FromData will find and optionally verify MongoDB secrets in a given set of bytes.
func (s Scanner) FromData(ctx context.Context, verify bool, data []byte) (results []detectors.Result, err error) {
	dataStr := string(data)

	matches := keyPat.FindAllStringSubmatch(dataStr, -1)

	for _, match := range matches {
		resMatch := strings.TrimSpace(match[1])

		s1 := detectors.Result{
			DetectorType: detectorspb.DetectorType_MongoDB,
			Raw:          []byte(resMatch),
		}

		if verify {
			timeout := s.timeout
			if timeout == 0 {
				timeout = defaultTimeout
			}
			err := verifyUri(resMatch, timeout)
			s1.Verified = err == nil
			if !isErrDeterminate(err) {
				s1.VerificationError = err
			}
		}
		results = append(results, s1)
	}

	return results, nil
}

func isErrDeterminate(err error) bool {
	switch e := err.(type) {
	case topology.ConnectionError:
		switch e.Unwrap().(type) {
		case *auth.Error:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func verifyUri(uri string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Disconnect(ctx)
	}()
	return client.Ping(ctx, readpref.Primary())
}

func (s Scanner) Type() detectorspb.DetectorType {
	return detectorspb.DetectorType_MongoDB
}
