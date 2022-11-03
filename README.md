![](https://user-images.githubusercontent.com/73197190/196969015-5c967955-ea75-4a51-ae55-7dd47155d402.png)

# APIs

![hedge_earth_identity_small](https://user-images.githubusercontent.com/73197190/199803427-1e5818d0-a925-462b-b8c1-27fb9588ba7b.png)

## HEDGE

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

## Shapshifter

* **Problem:** Traditional databases are clunky, complex and consume a large amount of resources and energy.
* **Solution:** Create a lightweight low resource storage without sacrificing availability, latency, performance.

**Resolution:**

Supercharge Github repos with RESTful APIs to easily commit JSON.

| Method | HEDGE.earth |
| ------------- | ------------- |
| GET  | https://edge.hedge.earth/emissionless/owner/repo/shapeshifter/path/id  |
| PUT  | https://edge.hedge.earth/emissionless/owner/repo/shapeshifter/path/id  |
| POST  | https://edge.hedge.earth/emissionless/owner/repo/shapeshifter/path/id  |

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

> Shapeshifter original intent was efficient cost effective means of storing [NxRx Data](https://v8.ngrx.io/guide/data) Entities. The API is being used exclusively with [Quell](https://github.com/rollthecloudinc/quell) our nonprofits carbon free low code editor on Reactive Angular. Quell relies heavily on NxRx Data to streamline managing data between the server and browser. Quell entities are currently hard coded into emissionless. Shapshifters goal is to enable a free flow of JSON of all entity types without needing to redeploy, modify emissionless.

# Media

Supercharge Github repos with API to upload media files
