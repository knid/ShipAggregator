# /auth/getToken

Test ve Canlı ortam bilgileri paylaşıldıktan sonra Postmanden Collection sekmesinde bulunan /auth/getToken servisinden token aldıktan sonra diğer servisler çağırarak işlem yapılabilir. Token kullanım süresi 60 dakikakadır. 60 dakika sonrasında tekrar token alınmalıdır.

Test Ortam URL: [https://integration-apitest.hepsijet.com](https://integration-apitest.hepsijet.com)

# OpenAPI definition

```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "HEPSIJET",
    "description": "Hepsijet Entegrasyon servisleri",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "https://integration-apitest.hepsijet.com"
    }
  ],
  "components": {
    "securitySchemes": {
      "basicAuth": {
        "type": "http",
        "scheme": "basic"
      }
    }
  },
  "tags": [
    {
      "name": "Hepsijet > Auth Servisi",
      "description": "Hepsijet Servislerini kullanmadan önce Authotication servisi kullanılarak token alınması gerekmektedir."
    }
  ],
  "paths": {
    "/auth/getToken": {
      "get": {
        "tags": [
          "Hepsijet > Auth Servisi"
        ],
        "summary": "/auth/getToken",
        "description": "Test ve Canlı ortam bilgileri paylaşıldıktan sonra Postmanden Collection sekmesinde bulunan /auth/getToken servisinden token aldıktan sonra diğer servisler çağırarak işlem yapılabilir. Token kullanım süresi 60 dakikakadır. 60 dakika sonrasında tekrar token alınmalıdır.\n\nTest Ortam URL: [https://integration-apitest.hepsijet.com](https://integration-apitest.hepsijet.com)",
        "security": [
          {
            "basicAuth": []
          }
        ],
        "parameters": [
          {
            "name": "Accept",
            "in": "header",
            "schema": {
              "type": "string"
            },
            "example": "application/json"
          }
        ],
        "responses": {
          "200": {
            "description": "Successful response",
            "content": {
              "application/json": {}
            }
          }
        }
      }
    }
  },
  "x-readme": {
    "explorer-enabled": true,
    "proxy-enabled": true
  },
  "_id": "63f741626d06f3007596eb34"
}
```