![](https://user-images.githubusercontent.com/73197190/196969015-5c967955-ea75-4a51-ae55-7dd47155d402.png)

Emissionless works alongside Climate Warrior to battle climate change on web. Emissionless provides a set of climate friendly APIs that can be used by Github users that install, authorize the Climate Warrior App for repositories which they would like to use the cross regional cloud hosted emissionless APIs.

# Shapshifter

Supercharge Github repos with RESTful APIs to easily commit JSON

| Method | Region: N. Virginia |
| ------------- | ------------- |
| GET  | https://us-east-1.emissionless.services/owner/repo/shapshifter/path/id  |
| POST  | https://us-east-1.emissionless.services/owner/repo/shapshifter/path/id  |

The POST body can be any valid JSON with an id property. The id property is used to distinguish unique json documents within the same provided path. The id in the path should match the id inside the json document body.

```javascript
{
  "id": "6f39a72a-6af3-4348-9158-7f111a6d0352"
  "title": "My first document"
}
```

# Media

Supercharge Github repos with API to upload media files.

# HEDGE

Bounce RESTful requests between data centers using renewable resources.
