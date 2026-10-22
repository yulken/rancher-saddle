package views

import (
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCredentialsForm_VisibleFields(t *testing.T) {
	m := NewCredentialsFormModel()

	assert.Equal(t, []int{
		credFieldType,
		credFieldName,
		credFieldAccessKey,
		credFieldSecretKey,
		credFieldRegion,
		credFieldSave,
	}, m.visibleFields())

	m.authType = core.AuthSSO

	assert.Equal(t, []int{
		credFieldType,
		credFieldName,
		credFieldProfile,
		credFieldRegion,
		credFieldSave,
	}, m.visibleFields(), "SSO hides the static key fields")
}

func TestCredentialsForm_MoveFocusSkipsHiddenFields(t *testing.T) {
	m := NewCredentialsFormModel()
	m.authType = core.AuthSSO
	m.focusIndex = 2 // profile

	m, _ = m.moveFocus(1)
	assert.Equal(t, credFieldRegion, m.currentField())

	m, _ = m.moveFocus(1)
	assert.Equal(t, credFieldSave, m.currentField())

	m, _ = m.moveFocus(1)
	assert.Equal(t, credFieldType, m.currentField(), "focus wraps around")
}

func TestCredentialsForm_FieldInputMapping(t *testing.T) {
	assert.Equal(t, inputName, fieldInput(credFieldName))
	assert.Equal(t, inputAccessKey, fieldInput(credFieldAccessKey))
	assert.Equal(t, inputSecretKey, fieldInput(credFieldSecretKey))
	assert.Equal(t, inputProfile, fieldInput(credFieldProfile))
	assert.Equal(t, inputRegion, fieldInput(credFieldRegion))
	assert.Equal(t, -1, fieldInput(credFieldType))
	assert.Equal(t, -1, fieldInput(credFieldSave))
}

func TestCredentialsForm_BuildCredential(t *testing.T) {
	t.Run("static", func(t *testing.T) {
		m := NewCredentialsFormModel()
		m.inputs[inputName].SetValue("prod")
		m.inputs[inputAccessKey].SetValue("AKIA123")
		m.inputs[inputSecretKey].SetValue("shhh")
		m.inputs[inputRegion].SetValue("us-east-1")

		cred := m.buildCredential()

		assert.Equal(t, core.AuthStatic, cred.Type)
		assert.Equal(t, "AKIA123", cred.AccessKey)
		assert.Empty(t, cred.Profile)
	})

	t.Run("sso", func(t *testing.T) {
		m := NewCredentialsFormModel()
		m.authType = core.AuthSSO
		m.inputs[inputName].SetValue("acme")
		m.inputs[inputProfile].SetValue("acme-prod")
		m.inputs[inputAccessKey].SetValue("should-be-ignored")
		m.inputs[inputRegion].SetValue("us-west-2")

		cred := m.buildCredential()

		assert.Equal(t, core.AuthSSO, cred.Type)
		assert.Equal(t, "acme-prod", cred.Profile)
		assert.Empty(t, cred.AccessKey, "stale static values are dropped")
		assert.Empty(t, cred.SecretKey)
	})
}
