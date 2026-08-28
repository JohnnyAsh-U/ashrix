package identity

import (
	"html/template"
	"strings"
)

func providerDisplayName(name string) string {
	switch strings.ToLower(name) {
	case "keycloak":
		return "Keycloak"

	case "google":
		return "Google"

	case "microsoft":
		return "Microsoft"

	case "azure":
		return "Microsoft"

	case "okta":
		return "Okta"

	case "github":
		return "GitHub"

	default:
		return name
	}
}

func providerIcon(name string) template.HTML {
	switch strings.ToLower(name) {

	case "keycloak":
		return template.HTML(`
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
				aria-hidden="true"
			>
				<circle
					cx="12"
					cy="12"
					r="10"
					fill="#4D4D4D"
				/>
				<path
					d="M9 7H15V9H11V11H14V13H11V15H15V17H9V7Z"
					fill="white"
				/>
			</svg>
		`)

	case "google":
		return template.HTML(`
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				xmlns="http://www.w3.org/2000/svg"
				aria-hidden="true"
			>
				<path
					fill="#4285F4"
					d="M21.35 12.23C21.35 11.57 21.29 10.93 21.16 10.31H12V14.1H17.22C17 15.32 16.3 16.35 15.25 17.04V19.54H18.42C20.27 17.84 21.35 15.34 21.35 12.23Z"
				/>
				<path
					fill="#34A853"
					d="M12 21.66C14.65 21.66 16.87 20.79 18.42 19.54L15.25 17.04C14.38 17.62 13.27 17.97 12 17.97C9.45 17.97 7.29 16.25 6.43 13.94H3.16V16.52C4.73 19.57 8 21.66 12 21.66Z"
				/>
				<path
					fill="#FBBC05"
					d="M6.43 13.94C6.24 13.37 6.13 12.76 6.13 12.14C6.13 11.52 6.24 10.91 6.43 10.34V7.76H3.16C2.5 9.05 2.12 10.51 2.12 12.14C2.12 13.77 2.5 15.23 3.16 16.52L6.43 13.94Z"
				/>
				<path
					fill="#EA4335"
					d="M12 6.31C13.44 6.31 14.73 6.81 15.75 7.79L18.49 5.05C16.86 3.53 14.65 2.62 12 2.62C8 2.62 4.73 4.71 3.16 7.76L6.43 10.34C7.29 8.03 9.45 6.31 12 6.31Z"
				/>
			</svg>
		`)

	case "microsoft", "azure":
		return template.HTML(`
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				xmlns="http://www.w3.org/2000/svg"
				aria-hidden="true"
			>
				<rect x="3" y="3" width="8" height="8" fill="#F25022"/>
				<rect x="13" y="3" width="8" height="8" fill="#7FBA00"/>
				<rect x="3" y="13" width="8" height="8" fill="#00A4EF"/>
				<rect x="13" y="13" width="8" height="8" fill="#FFB900"/>
			</svg>
		`)

	case "github":
		return template.HTML(`
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="currentColor"
				xmlns="http://www.w3.org/2000/svg"
				aria-hidden="true"
			>
				<path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.38 7.85 10.9.57.1.78-.25.78-.55v-2.13c-3.19.69-3.86-1.54-3.86-1.54-.52-1.33-1.27-1.69-1.27-1.69-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.67 1.25 3.32.96.1-.74.4-1.25.73-1.54-2.55-.29-5.23-1.28-5.23-5.69 0-1.26.45-2.29 1.18-3.1-.12-.29-.51-1.47.11-3.06 0 0 .96-.31 3.15 1.18A10.9 10.9 0 0 1 12 5.85c.97 0 1.94.13 2.85.37 2.19-1.49 3.15-1.18 3.15-1.18.62 1.59.23 2.77.11 3.06.73.81 1.18 1.84 1.18 3.1 0 4.42-2.69 5.4-5.25 5.68.41.36.78 1.07.78 2.16v3.2c0 .3.21.66.79.55A11.5 11.5 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5Z"/>
			</svg>
		`)

	default:
		return template.HTML(`
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
				aria-hidden="true"
			>
				<circle
					cx="12"
					cy="12"
					r="9"
					stroke="#6B7280"
					stroke-width="2"
				/>
				<path
					d="M12 8V12L14.5 14.5"
					stroke="#6B7280"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		`)
	}
}