# /deleteDeliveryAdvance/{customerOrderId}

Market Place kurgusu ile /advance/sendDeliveryAdvance/v2 servisi oluşturulan depo kaydının (Merchant Depo, Mağaza ) kaydının silmesi için kullanılan servistir.

URL kısmına orderid değeri yazılarak silinmelidir.

StartFragment

https://integration-apitest.hepsijet.com/rest/advance/deleteDeliveryAdvance/HJET123456

EndFragment

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
  "tags": [
    {
      "name": "Hepsijet > Gonderi Servisleri"
    }
  ],
  "paths": {
    "/rest/advance/deleteDeliveryAdvance/125": {
      "post": {
        "tags": [
          "Hepsijet > Gonderi Servisleri"
        ],
        "summary": "/deleteDeliveryAdvance/{customerOrderId}",
        "description": "Market Place kurgusu ile /advance/sendDeliveryAdvance/v2 servisi oluşturulan depo kaydının (Merchant Depo, Mağaza ) kaydının silmesi için kullanılan servistir.\n\nURL kısmına orderid değeri yazılarak silinmelidir.\n\nStartFragment\n\nhttps://integration-apitest.hepsijet.com/rest/advance/deleteDeliveryAdvance/HJET123456\n\nEndFragment",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{  \\r\\n   \\\"deleteReason\\\":\\\"IPTAL\\\"\\r\\n}\\r\\n\\r\\n\""
              }
            }
          }
        },
        "parameters": [
          {
            "name": "X-Auth-Token",
            "in": "header",
            "schema": {
              "type": "string"
            },
            "example": "e0d673df-63b8-4d56-a37e-056273316748"
          },
          {
            "name": "Content-Type",
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