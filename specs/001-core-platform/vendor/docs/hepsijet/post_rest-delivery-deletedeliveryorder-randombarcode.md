# /deleteDeliveryOrder/{deliveryNo}

/deleteDeliveryOrder  
● Gönderi iş ortağının deposundan alınmadan önce, henüz ilk statüsündeyken bu servis ile gönderi silinebilir.  
● Servis kullanımı ile önceden oluşturulan gönderinin kaydı HepsiJET tarafında ‘DELETED’ statüsüne çekilir.

StartFragment

URL alanına barcode bilgisini yazarak silme işlemi yapılabilir.

https://integration-apitest.hepsijet.com/rest/delivery/deleteDeliveryOrder/**HJET123456789**

EndFragment

Body içerisinde;

{  
"deleteReason":"IPTAL"  
}

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
    "/rest/delivery/deleteDeliveryOrder/{randomBarcode}": {
      "post": {
        "tags": [
          "Hepsijet > Gonderi Servisleri"
        ],
        "summary": "/deleteDeliveryOrder/{deliveryNo}",
        "description": "/deleteDeliveryOrder  \n● Gönderi iş ortağının deposundan alınmadan önce, henüz ilk statüsündeyken bu servis ile gönderi silinebilir.  \n● Servis kullanımı ile önceden oluşturulan gönderinin kaydı HepsiJET tarafında ‘DELETED’ statüsüne çekilir.\n\nStartFragment\n\nURL alanına barcode bilgisini yazarak silme işlemi yapılabilir.\n\nhttps://integration-apitest.hepsijet.com/rest/delivery/deleteDeliveryOrder/**HJET123456789**\n\nEndFragment\n\nBody içerisinde;\n\n{  \n\"deleteReason\":\"IPTAL\"  \n}",
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
          },
          {
            "name": "randomBarcode",
            "in": "path",
            "schema": {
              "type": "string"
            },
            "required": true
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