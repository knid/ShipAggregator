# /generateZplBarcode/{barcode}/{totalParcel}

sendDeliveryOrderEnhanced ve sendDeliveryOrder gönderilen siparişleri farklı zamanlarda ZPL çıktısı alınmak istendiğinden generateZplBarcode/BARCODEKODU/Totalparcel değerini yazarak çıktı alınabilir. Kaç adet çıktı alınmak isteniyorsa Totalparcel değerine o kadar yazılmaldır.

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
      "name": "Hepsijet"
    }
  ],
  "paths": {
    "/delivery/generateZplBarcode/FPR2000000000/1": {
      "get": {
        "tags": [
          "Hepsijet"
        ],
        "summary": "/generateZplBarcode/{barcode}/{totalParcel}",
        "description": "sendDeliveryOrderEnhanced ve sendDeliveryOrder gönderilen siparişleri farklı zamanlarda ZPL çıktısı alınmak istendiğinden generateZplBarcode/BARCODEKODU/Totalparcel değerini yazarak çıktı alınabilir. Kaç adet çıktı alınmak isteniyorsa Totalparcel değerine o kadar yazılmaldır.",
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