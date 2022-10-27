![](https://user-images.githubusercontent.com/73197190/196969015-5c967955-ea75-4a51-ae55-7dd47155d402.png)

# HEDGE

1. Problem: API requests contribute to 83% of web carbon emissions.
2. Solution: Maximize amount of clean energy used to fulfill API requests.



3. Resolution:

To this end our contribution to reducing web carbon begins with generating a periodical [renewables report](https://store.hedge.earth/renewable-report/report.json) of regional grid intensity levels across the globe from the carbon aware api for the next 5 minutes. The generated renewable report is used to redirect API requests to data centers within regions that are using the lowest carbon intense power sources. The API requests are redirected based on reported intensity levels inside the renewable report. Creating one of the worlds first known open source, hosted carbon aware reverse proxies maximizing renewables usage on the global scale for processing API requests.

> Reverse proxy: An application that sits in front of back-end applications and forwards client requests to those applications. Reverse proxies help increase scalability, performance, resilience, security and NOW clean energy use.

# Shapshifter

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
