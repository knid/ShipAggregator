# /sendDeliveryOrder

sendDeliveryOrderEnhanced servisindeki gibi yine senddeliveryOrder servisi gönderi iletim servisidir. Bu servisle 41 ve üzeri desi XL gönderilerin hepsijete iletilmesini sağlanmaktadır.

Payload içerisinde serviceType "TMH" olarak iletilmektedir.

sendDeliveryOrderEnhanced servisndeki payload kaydı ile birebir ayndır.

"serviceType":  
\[  
"TMH"  
\],

Örnek Request;

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
    "/delivery/sendDeliveryOrder": {
      "post": {
        "tags": [
          "Hepsijet > Gonderi Servisleri"
        ],
        "summary": "/sendDeliveryOrder",
        "description": "sendDeliveryOrderEnhanced servisindeki gibi yine senddeliveryOrder servisi gönderi iletim servisidir. Bu servisle 41 ve üzeri desi XL gönderilerin hepsijete iletilmesini sağlanmaktadır.\n\nPayload içerisinde serviceType \"TMH\" olarak iletilmektedir.\n\nsendDeliveryOrderEnhanced servisndeki payload kaydı ile birebir ayndır.\n\n\"serviceType\":  \n\\[  \n\"TMH\"  \n\\],\n\nÖrnek Request;",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{\\r\\n  \\\"company\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"FAPRIKA\\\",\\r\\n\\\"abbreviationCode\\\": \\\"FPRKA\\\"\\r\\n},\\r\\n\\\"serviceType\\\": \\r\\n[\\r\\n\\\"TMH\\\" \\r\\n], \\r\\n\\\"delivery\\\": \\r\\n{\\r\\n\\\"deliveryDateOriginal\\\": \\\"2022-12-02\\\",    \\r\\n\\\"customerDeliveryNo\\\": \\\"FPR1400000000000\\\",\\r\\n\\\"customerOrderId\\\": \\\"FPR1400000000000\\\",\\r\\n\\\"deliveryType\\\": \\\"RETAIL\\\",\\r\\n\\\"totalParcels\\\": 1,\\r\\n\\\"desi\\\": 50,\\r\\n\\\"product\\\": \\r\\n{\\r\\n\\\"productCode\\\": \\\"HX_STD\\\"\\r\\n},\\r\\n\\\"senderAddress\\\": \\r\\n{\\r\\n\\\"companyAddressId\\\": \\\"faprika_ankra_address\\\",\\r\\n\\\"country\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Türkiye\\\"\\r\\n},\\r\\n\\\"city\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Ankara\\\"\\r\\n},\\r\\n\\\"town\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Yenimahalle\\\"\\r\\n},\\r\\n\\\"district\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Demetlale\\\"\\r\\n},\\r\\n\\\"addressLine1\\\": \\\"Demetlale Mahallesi 405. Cd. No:70 06200 Yenimahalle/Ankara\\\"\\r\\n},\\r\\n\\\"recipientAddress\\\": \\r\\n{\\r\\n\\\"companyAddressId\\\": \\\"FPR1400000000000\\\",\\r\\n\\\"country\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Türkiye\\\"\\r\\n},\\r\\n\\\"city\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Bursa\\\"\\r\\n},\\r\\n\\\"town\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Osmangazi\\\"\\r\\n},\\r\\n\\\"district\\\": \\r\\n{\\r\\n\\\"name\\\": \\\"Güneştepe\\\"\\r\\n},\\r\\n\\\"addressLine1\\\": \\\"Güneştepe Mh. 771 sok. No:1 D:5 Osmangazi Bursa\\\"\\r\\n},\\r\\n\\\"receiver\\\": \\r\\n{\\r\\n\\\"companyCustomerId\\\": \\\"FPR1400000000000\\\",\\r\\n\\\"firstName\\\": \\\"MEHMET\\\",\\r\\n\\\"lastName\\\": \\\"TUFAN\\\",\\r\\n\\\"phone1\\\": \\\"5322103019\\\"\\r\\n},\\r\\n\\\"recipientPerson\\\": \\\"MEHMET KÜRŞAD TUFAN\\\",\\r\\n\\\"recipientPersonPhone1\\\": \\\"5494380768\\\",\\r\\n\\\"currentXDock\\\": \\r\\n{\\r\\n\\\"abbreviationCode\\\": \\\"FPRKA_ANKRA\\\"\\r\\n    }\\r\\n  }\\r\\n}\""
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