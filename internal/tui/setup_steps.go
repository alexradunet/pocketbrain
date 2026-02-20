package tui

// setupStep identifies each step in the setup wizard.
type setupStep int

const (
	stepProvider setupStep = iota
	stepKronkCatalog
	stepKronkModelSelect
	stepKronkDownloadConfirm
	stepKronkDownload
	stepModel
	stepAPIKey
	stepWhatsAppEnable
	stepWhatsAppAuthDir
	stepWhatsAppNumber
	stepWorkspacePath
	stepWebDAVEnable
	stepWebDAVAddr
	stepSaving
	stepDone
)

// stepTitle returns a human-readable title for each step.
func stepTitle(s setupStep) string {
	switch s {
	case stepProvider:
		return "LLM Provider"
	case stepKronkCatalog:
		return "Fetching Kronk Catalog..."
	case stepKronkModelSelect:
		return "Select Kronk Model"
	case stepKronkDownloadConfirm:
		return "Download Model?"
	case stepKronkDownload:
		return "Downloading Model..."
	case stepModel:
		return "Model Name"
	case stepAPIKey:
		return "API Key"
	case stepWhatsAppEnable:
		return "Enable WhatsApp?"
	case stepWhatsAppAuthDir:
		return "WhatsApp Auth Directory"
	case stepWhatsAppNumber:
		return "WhatsApp Allowed Number"
	case stepWorkspacePath:
		return "Workspace Path"
	case stepWebDAVEnable:
		return "Enable WebDAV?"
	case stepWebDAVAddr:
		return "WebDAV Listen Address"
	case stepSaving:
		return "Saving Configuration..."
	case stepDone:
		return "Setup Complete"
	default:
		return ""
	}
}

// nextStep determines the transition from the current step based on collected values.
func nextStep(current setupStep, values map[string]string) setupStep {
	switch current {
	case stepProvider:
		if values["PROVIDER"] == "kronk" {
			return stepKronkCatalog
		}
		return stepModel

	case stepKronkCatalog:
		return stepKronkModelSelect

	case stepKronkModelSelect:
		return stepKronkDownloadConfirm

	case stepKronkDownloadConfirm:
		if values["_download_now"] == "true" {
			return stepKronkDownload
		}
		return stepWhatsAppEnable

	case stepKronkDownload:
		return stepWhatsAppEnable

	case stepModel:
		return stepAPIKey

	case stepAPIKey:
		return stepWhatsAppEnable

	case stepWhatsAppEnable:
		if values["ENABLE_WHATSAPP"] == "true" {
			return stepWhatsAppAuthDir
		}
		return stepWorkspacePath

	case stepWhatsAppAuthDir:
		return stepWhatsAppNumber

	case stepWhatsAppNumber:
		return stepWorkspacePath

	case stepWorkspacePath:
		return stepWebDAVEnable

	case stepWebDAVEnable:
		if values["WEBDAV_ENABLED"] == "true" {
			return stepWebDAVAddr
		}
		return stepSaving

	case stepWebDAVAddr:
		return stepSaving

	case stepSaving:
		return stepDone

	default:
		return stepDone
	}
}
