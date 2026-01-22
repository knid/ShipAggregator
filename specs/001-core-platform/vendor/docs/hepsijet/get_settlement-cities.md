# /settlement/cities

Token servisi çağrrıldıktan sonra GET işlemi sorgulama yapılır. Dağıtım alanlarımızı il bazında kontrol edilebileceği servistir. settlement/cities servisinin herhnagi bir body veya api sorgusu bulunmamaktadır. Sorgu sonucunda İl ve ID şeklinde tutulmaktadır. Adres mapleme sürecinde bu servisin kullanılması önerilmektedir.

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
      "name": "Hepsijet > Settlement Servisleri",
      "description": "Hepsijet Aktif Dağıtım alanlarını anlık olarak sorgulamak için kullanılan servislerdir."
    }
  ],
  "paths": {
    "/settlement/cities": {
      "get": {
        "tags": [
          "Hepsijet > Settlement Servisleri"
        ],
        "summary": "/settlement/cities",
        "description": "Token servisi çağrrıldıktan sonra GET işlemi sorgulama yapılır. Dağıtım alanlarımızı il bazında kontrol edilebileceği servistir. settlement/cities servisinin herhnagi bir body veya api sorgusu bulunmamaktadır. Sorgu sonucunda İl ve ID şeklinde tutulmaktadır. Adres mapleme sürecinde bu servisin kullanılması önerilmektedir.",
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