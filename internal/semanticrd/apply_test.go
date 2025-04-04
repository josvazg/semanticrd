package semanticrd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/josvazg/semanticrd/internal/semanticrd"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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
			input: `apiVersion: atlas.generated.mongodb.com/v1
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
			want: `apiVersion: v1
kind: Secret
metadata:
  name: atlasthirdpartyintegration-sample-secret
data:
  licenseKey: 3c41a2b6fd5e
  readToken: 178234y1hdwu1
  writeToken: 219u42390r8fpe2hf
---
apiVersion: atlas.generated.mongodb.com/v1
kind: AtlasThirdPartyIntegration
metadata:
  name: atlasthirdpartyintegration-sample
spec:
  v20231115:
    identifier:
      id: 12345567890
    references:
      groupId: 12345667890
    type: NEW_RELIC
    accountId: 1a2b3c4d5e6f
    credentialsSecret: atlasthirdpartyintegration-sample-secret
`,
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			out := bytes.NewBufferString("")
			err := semanticrd.Apply(out, bytes.NewBufferString(tc.input), bytes.NewBufferString(tc.semantics))
			want := reencode(t, tc.want)
			got := reencode(t, out.String())
			assert.Equal(t, want, got)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func reencode(t *testing.T, ymls string) string {
	objs := []map[string]interface{}{}
	for _, yml := range strings.Split(ymls, "\n---\n") {
		obj := map[string]interface{}{}
		err := yaml.Unmarshal(([]byte)(yml), &obj)
		if err != nil {
			t.Fatalf("could not unmarshal %q: %v", yml, err)
		}
		objs = append(objs, obj)
	}
	outs := []string{}
	for _, obj := range objs {
		out, err := yaml.Marshal(obj)
		if err != nil {
			t.Fatalf("could not marshal %v: %v", obj, err)
		}
		outs = append(outs, string(out))
	}
	return strings.Join(outs, "\n---\n")
}
