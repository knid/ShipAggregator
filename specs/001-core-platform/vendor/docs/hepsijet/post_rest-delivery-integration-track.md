# /delivery/integration/track

sendDeliveryOrderEnhanced ve sendDeliveryOrder serviskerinde payload içerisinde iletilen customerDeliveryNo (Barcode) ile integration/track servisiyle gönderinin 3 ana statüsü ve gönderi takip linki oluşmaktadır.

Gönderi herhangi bir operasyonel yani toplama işlemi yapılmadan sorgulandığında barcode statüsü "OK" olarak dönmektedir.

Barcode sorgulama işlemi;

{

"barcodes": \[

"HJT1000000000",

"HJT1000000001",

"HJT1000000002",

"HJT1000000003"

\],

"isTrackAdded": true

}

Sorgulama sonucu;

{  
"status": "OK",  
"data": \[  
{  
"barcode": "HJT1000000001",  
"trackingUrl": "[https://hepsijet.com/gonderi-takibi/HJT1000000001"](https://hepsijet.com/gonderi-takibi/HJT1000000001),  
"details": \[  
{  
"transactionLocation": "Kitapsan_cukurova Toplama Merkezi",  
"transactionDate": "2023-02-14T18:00:35+03:00",  
"integrationStatus": "ACCEPTED"  
},  
{  
"transactionLocation": "Dağıtım Aracında",  
"transactionDate": "2023-02-14T18:01:07+03:00",  
"integrationStatus": "OUT_FOR_DELIVERY"  
},  
{  
"transactionLocation": "Alıcıda",  
"transactionDate": "2023-02-14T18:01:09+03:00",  
"integrationStatus": "DELIVERED",  
"deliveredPerson": "Mehmet Kürşad"  
}  
\]  
}  
\]  
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
      "name": "Hepsijet > Tracking Servisleri"
    }
  ],
  "paths": {
    "/rest/delivery/integration/track": {
      "post": {
        "tags": [
          "Hepsijet > Tracking Servisleri"
        ],
        "summary": "/delivery/integration/track",
        "description": "sendDeliveryOrderEnhanced ve sendDeliveryOrder serviskerinde payload içerisinde iletilen customerDeliveryNo (Barcode) ile integration/track servisiyle gönderinin 3 ana statüsü ve gönderi takip linki oluşmaktadır.\n\nGönderi herhangi bir operasyonel yani toplama işlemi yapılmadan sorgulandığında barcode statüsü \"OK\" olarak dönmektedir.\n\nBarcode sorgulama işlemi;\n\n{\n\n\"barcodes\": \\[\n\n\"HJT1000000000\",\n\n\"HJT1000000001\",\n\n\"HJT1000000002\",\n\n\"HJT1000000003\"\n\n\\],\n\n\"isTrackAdded\": true\n\n}\n\nSorgulama sonucu;\n\n{  \n\"status\": \"OK\",  \n\"data\": \\[  \n{  \n\"barcode\": \"HJT1000000001\",  \n\"trackingUrl\": \"[https://hepsijet.com/gonderi-takibi/HJT1000000001\"](https://hepsijet.com/gonderi-takibi/HJT1000000001),  \n\"details\": \\[  \n{  \n\"transactionLocation\": \"Kitapsan_cukurova Toplama Merkezi\",  \n\"transactionDate\": \"2023-02-14T18:00:35+03:00\",  \n\"integrationStatus\": \"ACCEPTED\"  \n},  \n{  \n\"transactionLocation\": \"Dağıtım Aracında\",  \n\"transactionDate\": \"2023-02-14T18:01:07+03:00\",  \n\"integrationStatus\": \"OUT_FOR_DELIVERY\"  \n},  \n{  \n\"transactionLocation\": \"Alıcıda\",  \n\"transactionDate\": \"2023-02-14T18:01:09+03:00\",  \n\"integrationStatus\": \"DELIVERED\",  \n\"deliveredPerson\": \"Mehmet Kürşad\"  \n}  \n\\]  \n}  \n\\]  \n}",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{\\r\\n  \\\"barcodes\\\": [\\r\\n    \\\"FPR1000000000\\\",\\r\\n    \\\"FPR1000000001\\\",\\r\\n    \\\"FPR1000000002\\\",\\r\\n    \\\"FPR1000000001\\\"\\r\\n  ],\\r\\n  \\\"isTrackAdded\\\": true\\r\\n}\""
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