![17](https://user-images.githubusercontent.com/73197190/202189020-3672deb5-9847-4531-8070-d5aad289e9d8.png)

# Initiatives

![hedge_earth_identity_small](https://user-images.githubusercontent.com/73197190/199803427-1e5818d0-a925-462b-b8c1-27fb9588ba7b.png)

HEDGE reimagines the way websites are built using the best sustainable technology solutions the industry has to offer.

> HEDGING (Software Development): Limiting software exposure to carbon by using cleanest energy globally available.

* Store: Repairing data storage for good.
  * Data storage that moves around the world using the cleanest energy.
* Proxy: Rewriting the web for good.
  * Reverse proxy bouncing traffic to data centers using cleanest energy.
* Track: Record and reduce for good. 
  * Record emissions providing actionable intelligance for reduction and offseting.

## Store

* **Problem:** Traditional data storage has limited access to clean energy.
* **Solution:** Provide data store that moves around the world using the cleanest energy.

**Resolution:**

Repair data storage for good providing REST APIs that move around world between cleanest energy grids to store data. Data is stored on Github using repositories rather than individual databases. Transactions are wrapped in electricity and carbon emissions scope 1,2,3 tracking/monitoring providing complete picture of emissions. Our REST API is distributed across all the regions below delegating cpu processing to the one with the lowest grid intensity.

AWS

| Location  | Domain | Mapping |
| ------------- | ------------- | ----------- |
| Montreal | https://ca-central-1.octostore.earth | canadacentral |
| Ashburn VA  | https://us-east-1.octostore.earth  | eastus |
| San Fransisco  | https://us-west-1.octostore.earth | westus |
| Dublin  | https://eu-west-1.octostore.earth | ukwest |
| London  | https://eu-west-2.octostore.earth | uknorth |
| Frankfurt | https://eu-central-1.octostore.earth | germanywestcentral |
| Stockholm | https://eu-north-1.octostore.earth | northeurope/swedencentral |

Azure


| Location  | Domain | Mapping |
| ------------- | ------------- | ----------- |
| Norway East | https://norway-hedge.azurewebsites.net | norwayeast |

Cloudflare

@todo

Cosmonic

@todo


### JSON API

Store JSON using the cleanest energy resources.

| Method | HEDGE.earth |
| ------------- | ------------- |
| GET  | https://edge.hedge.earth/octostore/owner/repo/shapeshifter/path/id  |
| PUT  | https://edge.hedge.earth/octostore/owner/repo/shapeshifter/path/id  |
| POST  | https://edge.hedge.earth/octostore/owner/repo/shapeshifter/path/id  |

> The emissionless API is the first carbon aware API being bounced to low intensity data centers using HEDGE.earth. You can follow in our footsteps by submitting a pull requests for your service to our [HEDGE objects dev repo](https://github.com/rollthecloudinc/hedge-objects/tree/dev/services). Once you have tested, verified HEDGE.earth works with your API submit a pull request to [HEDGE objects prod repo](https://github.com/rollthecloudinc/hedge-objects-prod/tree/master/services). See our [emissionless.json](https://store.hedge.earth/services/emissionless.json) service schema for reference and [_schema.json](https://store.hedge.earth/services/_schema.json) for json schema defination of a HEDGE service. Valid regions can be found in the [regions json file](https://store.hedge.earth/regions/regions.json).

The POST body can be any valid JSON with an id property. The id property is used to distinguish unique json documents within the same provided path. The id of the parameter should match the id inside the json document body.

```javascript
{
  "id": "6f39a72a-6af3-4348-9158-7f111a6d0352"
  "title": "My first document"
}
```

JSON Documents comitted via the shapshifter API will have the user id added automatically of the authenticated user that made the change.


Example Response:
```javascript
{
  "id": "6f39a72a-6af3-4348-9158-7f111a6d0352"
  "title": "My first document",
  "userId": "cc149bd7-83ef-47c5-a397-eb0eb0068e0d"
}
```

View [use cases](https://github.com/rollthecloudinc/emissionless/wiki/Shapshifter-Use-Cases) for more specific examples.

Future Features:
* Validation
  * Repository owners will be able to provide [JSON schema](https://json-schema.org/) files that are used to validate entities before commiting. Entities that fail validation will not be comitted producing error messaging instead.
* Search
  * JSON documents become searchable using [Open Search](https://opensearch.org/) API and dashboards. Climate Warrior App installers will have access to both Open Search API and their own dashboards.
  * Index schemas will also be customizable including defining the schema documents will use for indexing.
* Notifications
  * Clients will be able to subscribe to various push notifications during the flow of saving data.
* Webhooks
  * Developers will be able to alter incoming and outgoing data using their own custom webhooks. Including implementing their own validation strategy when JSON Schema doesn't fit the bill.

### File API

Store Media and other files under 100MB using cleanest energy resources.

### Big File API

Store Media and other files over 100MB using cleanest energy resources.

## Proxy

* **Problem:** API requests contribute to 83% of web carbon emissions
* **Solution:** Maximize amount of clean energy used to fulfill API requests

**Resolution:** 

To this end our contribution to reducing web carbon begins with generating a [periodical](https://github.com/rollthecloudinc/hedge-objects-prod/commits/master/renewable-report) [renewables report](https://store.hedge.earth/renewable-report/report.json) of regional grid intensity levels across the globe from the [Green Software Foundation](https://greensoftware.foundation/) [carbon aware api](https://carbon-aware-api.azurewebsites.net/swagger/index.html) for the next 5 minutes. The generated renewable report is used to redirect API requests to data centers within regions that are using the lowest carbon intense [power sources](https://www.watttime.org/explorer/). The API requests are redirected based on reported intensity levels inside the renewable report. Rewriting the definition of a reverse proxy to include the advantage of maximizing clean energy use.

Reverse proxy: 

An application that sits in front of back-end applications and forwards client requests to those applications. Reverse proxies help increase scalability, performance, resilience, security and clean energy use. The HEDGE reverse proxy url is below for both the prod and dev environments. Request to register services will be promoted to prod once tested on dev via pull requests.

| Method | Endpoint | Environment |
| ------------- | ------------- |---------------|
| ANY  | https://edge.hedge.earth/{service}/{proxy+}  | Production |
| ANY  | https://hedgeedgex.druidcloud.dev/{service}/{proxy+}  | Development |

Javascript Package:

For CORs compatible APIs and Websockets the HEDGE proxy can be bypassed opting to use the [HEDGE JavaScript package](https://github.com/rollthecloudinc/emissionless/pkgs/npm/hedge) instead.The HEDGE JavaScript package carries out the same operations as the API but without wasting a network trip. Custom services can also be used without registering them as part of pull requests.

Import HEDGE
```javascript
import { hedge } from '@rollthecloudinc/hedge';
```

Climate friendly POST request for service.
```javascript
const method = 'POST';
const body = { id: "b83f9717-ab11-4e0f-a058-872af9bbe3ed", title: "Test Add", price: 50 };
const h = await hedge({ service: 'emissionless' });
const res = await h.bounce('/rollthecloudinc/classifieds/shapeshifter/ads/b83f9717-ab11-4e0f-a058-872af9bbe3ed', { method, body })
```

> Hedge.bounce() has the same interface as [fetch](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch) except the protocol (https://) and domain name are omitted.

The region that was used to make the request and comparisions between other regions carbon intensity levels can also be made.

```javascript
const { region } = await h.region();
const { regionDocument } = await region.document();
console.log('region used', regionDocument.region);
```

```javascript
const { difference } = await region.compare({ region: 'useast' })
console.log('difference in carbon intensity between region useast and the region used to carry out request.', difference)
```

The complete HEDGE Javascript API has been documented in our [demo repository](https://github.com/rollthecloudinc/hedge-demo/blob/master/src/index.js). The HEDGE javaScript API source code can be found under [/lib/hedge](https://github.com/rollthecloudinc/emissionless/tree/master/lib/hedge).

**Impact**

HEDGE has HUGE potential reach and potential CO2 reduction impact, with over 90% of Developers using APIs and emitting 16 million tonnes of CO2 generated each year. HEDGE could be very simply incorporated by hundreds of thousands of APIs to reduce their emissions, aggregating into a large global reduction.

**Future Features**
* Advertise & Promote Amount of carbon being saved
  * Electricity usage and carbon emission tracking and monitoring.
  * Website to submit servcies, track and monitor emissions globally, per org, per servce, region, etc.
* Postman plugin (50 million APIs)
* Serverless Framework plugin to incorporate into AWS API Gateway provisioning
* Nginx and HaProxy extension
* Other configuration as code platforms

## Track

* **Problem:** No real time monitoring for SCI that adjusts based on grid intensity of serverless AWS lambdas exists.
* **Solution:** Build real time monitoring for SCI backed by powerful search and intelligance engine.

**Resolution:**

Lambda created as AWS log subscriber that runs after each execution of lambdas within an account. The lambda collects key info and metrics of each lambda run including electricity usage and carbon production. Carbon production is calculated based on the Cloud Jewels algorithm but adjusted for real time grid intensity which region the Lambda is being executed. The info is stored inside AWS Open Search where we than apply analysys using machine learning and AI to lower the overall carbon output.

# Support

Roll the Cloud INC. is a registered 501(c)3 nonprofit US charity with the mission to exhile carbon from the web.


## Contact Us

* [Email](mailto:hi@rollthecloud.com)

## Follow Us

* [Twitter](https://twitter.com/rollthecloud)
* [Facebook](https://www.facebook.com/rollthecloud)

## Contribute

* [Github](https://github.com/rollthecloudinc)
* [Paypal Giving Fund](https://www.paypal.com/fundraiser/charity/4587641)
* [Climate Warrior Apparel](https://www.bonfire.com/store/climatewarrior/)
