implementation of an OIDC adapter (pkce:prononciation of PKCE) to bridge compatibility gaps between Login.gov and AWS Cognito and centralize external auth and/or support internal legacy user databases without requiring explicit migrations. The following changes are included:

**Pulumi Infrastructure:**

Created DynamoDB tables for state, code verifier, and redirect URI tracking.
Deployed API Gateway with Lambda-backed endpoints (

- /login
- /callback
- /token
- /user

Configured Lambda functions with required IAM permissions and environment variables.

**Lambda Functions:**

- loginLambda: Handles the /login endpoint to initiate the OIDC flow with PKCE (Proof Key for Code Exchange) and redirects users to Login.gov.
- callbackLambda: Processes Login.gov responses at the /callback endpoint, exchanges authorization codes for tokens, and redirects users back to Cognito.
- tokenLambda: Implements the /token endpoint for exchanging authorization codes for tokens.
- userLambda: Retrieves user profile information from Cognito's /userinfo endpoint.

**Features:**

- Secure state management with short-lived TTL in DynamoDB for mitigating CSRF and replay attacks.
- PKCE-based implementation for enhanced security in OIDC flows.
- Dynamic discovery of Cognito endpoints using the OpenID Connect discovery document.
- Comprehensive error handling and logging for debugging.

**Testing Notes:**

- Deployed and tested the full OIDC flow end-to-end, including Login.gov authentication and Cognito redirection.
- Verified DynamoDB entries (state, authorization codes) and their TTL behavior.
- Ensured error propagation for invalid or expired tokens.
- Impact: This implementation enables seamless integration with Login.gov while maintaining compatibility with AWS Cognito, providing a robust and secure authentication solution for our monorepo.