package semanticrd_test

import (
	"bytes"
	"testing"

	"github.com/josvazg/semanticrd/internal/semanticrd"
	"github.com/stretchr/testify/assert"
)

func TestApply(t *testing.T) {
	for _, tc := range []struct {
		title     string
		input     string
		semantics string
		want      string
		wantErr   string
	}{
		{
			title: "sample 1",
			input: `apiVersion: atlas.pre-generated.mongodb.com/v1
kind: AtlasThirdPartyIntegration
metadata:
  name: atlasthirdpartyintegration-sample
spec:
  v20231115:
    id: 12345567890
    groupId: 12345667890
    type: NEW_RELIC
    accountId: 1a2b3c4d5e6f
    licenseKey: 3c41a2b6fd5e
    readToken: 178234y1hdwu1
    writeToken: 219u42390r8fpe2hf
`,
            semantics: `group: atlas.generated.mongodb.com
versions:
  - v20231115
  - v20241113
globals:
  - type: ID
    id:
      path: ".id"
  - type: REFERENCE
    ref:
      path: ".groupId"
      parentKind: group
overrides:
  AtlasThirdPartyIntegration:
    - type: SECRET
      secret:
        name: "credentialsSecret"
        fields:
        - path: ".licenseKey"
        - path: ".readToken"
        - path: ".writeToken"
`,
			want: `apiVersion: atlas.generated.mongodb.com/v1
kind: AtlasThirdPartyIntegration
metadata:
  name: atlasthirdpartyintegration-sample
spec:
  id: 12345567890 # ID
  groupId: 12345667890 # Ref Kind: group
  type: NEW_RELIC
  newRelic:
    accountId: 1a2b3c4d5e6f
    credentialsSecret: new-relic-secret`,
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			out := bytes.NewBufferString("")
			err := semanticrd.Apply(out, bytes.NewBufferString(tc.input), bytes.NewBufferString(tc.semantics))
			assert.Equal(t, tc.want, out.String())
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
