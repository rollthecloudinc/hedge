const https = require("https");

exports.handler = async (event, context) => {
  try {
    // Load environment variables for Azure
    const subscriptionId = process.env.SUBSCRIPTION_ID;
    const resourceGroupName = "gosite";
    let staticWebAppName = event.Input.entity.repoName + "-dev";
    const location = "eastus2";
    const githubRepoUrl = "https://github.com/rollthecloudinc/" + event.Input.entity.repoName + "-build";
    let githubBranch = "dev";
    const clientId =  process.env.SERVICE_PRINCIPLE_CLIENT_ID;
    const clientSecret =  process.env.SERVICE_PRINCIPLE_CLIENT_SECRET;
    const tenantId =  process.env.SERVICE_PRINCIPLE_TENANT_ID;

    // Construct API URL for Azure Resource Manager (ARM)
    let armUrl = `https://management.azure.com/subscriptions/${subscriptionId}/resourceGroups/${resourceGroupName}/providers/Microsoft.Web/staticSites/${staticWebAppName}?api-version=2022-09-01`;

    // Prepare the request payload for the Static Web App
    let requestBody = JSON.stringify({
      location: location,
      sku: { name: "Free" },
      properties: {
        repositoryUrl: githubRepoUrl,
        branch: githubBranch,
        buildProperties: {
          appLocation: "",
          apiLocation: "",
          outputLocation: ""
        }
      }
    });

    // Step 1: Authenticate and obtain Azure token
    let token = await getAzureToken(tenantId, clientId, clientSecret);

    // Step 2: Create Static Web App in Azure
    let devResult = await makeHttpsRequest(armUrl, "PUT", requestBody, token);

    // Step 3: Retrieve deployment token for the Static Web App
    let devDeploymentToken = await getDeploymentToken(subscriptionId, resourceGroupName, staticWebAppName, token);

    githubBranch = "master"
    staticWebAppName = event.Input.entity.repoName + "-prod";
    armUrl = `https://management.azure.com/subscriptions/${subscriptionId}/resourceGroups/${resourceGroupName}/providers/Microsoft.Web/staticSites/${staticWebAppName}?api-version=2022-09-01`;

    requestBody = JSON.stringify({
      location: location,
      sku: { name: "Free" },
      properties: {
        repositoryUrl: githubRepoUrl,
        branch: githubBranch,
        buildProperties: {
          appLocation: "",
          apiLocation: "",
          outputLocation: ""
        }
      }
    });

    // Step 1: Authenticate and obtain Azure token
    token = await getAzureToken(tenantId, clientId, clientSecret);

    // Step 2: Create Static Web App in Azure
    const prodResult = await makeHttpsRequest(armUrl, "PUT", requestBody, token);

    // Step 3: Retrieve deployment token for the Static Web App
    const prodDeploymentToken = await getDeploymentToken(subscriptionId, resourceGroupName, staticWebAppName, token);

    // Return object with hostname and deployment token for Step Function
    return {
      devHostname: devResult.properties.defaultHostname,
      devDeploymentToken: devDeploymentToken,
      prodHostname: prodResult.properties.defaultHostname,
      prodDeploymentToken: prodDeploymentToken
    };
  } catch (err) {
    console.error("Error:", JSON.stringify(err, null, 2));
    throw err; // Let Step Function handle the error
  }
};

// Helper function for Azure authentication
async function getAzureToken(tenantId, clientId, clientSecret) {
  const requestBody = new URLSearchParams({
    grant_type: "client_credentials",
    client_id: clientId,
    client_secret: clientSecret,
    resource: "https://management.azure.com/"
  }).toString();

  const tokenUrl = `https://login.microsoftonline.com/${tenantId}/oauth2/token`;

  const tokenResponse = await makeHttpsRequest(tokenUrl, "POST", requestBody, null, "application/x-www-form-urlencoded");
  return tokenResponse.access_token;
}

// Helper function to retrieve deployment token for the Static Web App
async function getDeploymentToken(subscriptionId, resourceGroupName, staticWebAppName, token) {
  const secretsUrl = `https://management.azure.com/subscriptions/${subscriptionId}/resourceGroups/${resourceGroupName}/providers/Microsoft.Web/staticSites/${staticWebAppName}/listSecrets?api-version=2022-09-01`;

  const secretsResponse = await makeHttpsRequest(secretsUrl, "POST", null, token);
  return secretsResponse.properties.apiKey; // Deployment token is in the 'apiKey' field
}

// Helper function to make HTTPS requests
function makeHttpsRequest(url, method, body, token = null, contentType = "application/json") {
  return new Promise((resolve, reject) => {
    const urlObj = new URL(url);

    const options = {
      hostname: urlObj.hostname,
      path: urlObj.pathname + urlObj.search,
      method: method,
      headers: {
        "Content-Type": contentType,
        "Content-Length": Buffer.byteLength(body || "")
      }
    };

    if (token) {
      options.headers["Authorization"] = `Bearer ${token}`;
    }

    const req = https.request(options, (res) => {
      let data = "";

      res.on("data", (chunk) => {
        data += chunk;
      });

      res.on("end", () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(JSON.parse(data));
        } else {
          console.error("HTTP request failed:", {
            statusCode: res.statusCode,
            responseBody: data
          });
          reject(new Error(`Request failed with status: ${res.statusCode}, body: ${data}`));
        }
      });
    });

    req.on("error", (err) => {
      reject(err);
    });

    if (body) {
      req.write(body);
    }

    req.end();
  });
}