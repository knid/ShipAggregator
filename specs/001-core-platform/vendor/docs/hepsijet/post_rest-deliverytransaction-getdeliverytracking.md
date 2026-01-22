# /deliveryTransaction/getDeliveryTracking

sendDeliveryOrderEnhanced ve sendDeliveryOrder serviskerinde payload içerisinde iletilen customerDeliveryNo (Barcode) ile getDeliveryTracking servisinden sorgulama yapılarak gönderiye ait hareketler sorgulama yapılabilir.

Toplu sorgulama da yapılabilir. Bir limit bulunmamaktadır.

Tek tek sorgulama;

{ "deliveries" : \[

{

"customerDeliveryNo" : "HJT1000000001"

}

\]}

Toplu Sorgulama;

{ "deliveries" : \[

{

"customerDeliveryNo" : "HJT1000000001",

},

{

"customerDeliveryNo" : "HJT1000000002"

}

\]

}

Sorgulama Sonucu;

{  
"status": "OK",  
"data": \[  
{  
"arrivalBranch": "Osmaniye Dağıtım Merkezi",  
"customerDeliveryNo": "HJT1000000001",  
"barcode": "HJT1000000001",  
"deliveryType": "RETAIL",  
"currentLocation": "Alıcıda",  
"lastTransaction": "Teslim edildi - Mehmet K**\* (Kendisi)",****"receiver": "Mehmet K**\*",****"sender": "YASIN_AKCA",****"recipientName": "Mehmet Kürşad T**",**"deliveryDateCurrent": "2023-02-14",**"deliverySlotCurrent": 1,****"deliveredPerson": "Mehmet K**\*",  
"deliveredDate": "2023-02-14T18:01:09+03:00",  
"deliverdPersonIdNo": "99999999990",  
"deliveryStatus": "DELIVERED",  
"operationStatus": "IN_RECIPIENT",  
"transactionType": "DELIVERY_ATTEMPT",  
"company": {  
"id": 7618701,  
"name": "YASIN_AKCA",  
"companyType": "CUSTOMER",  
"abbreviationCode": "YASIN_AKCA",  
"corporateName": "YASİN AKÇA",  
"companyExtension": {  
"id": 388596,  
"ringDepartureEmailNeeded": false,  
"reportEmail": false,  
"printBarcode": false,  
"companyFlag": false,  
"companyAlternativeReceiverFlag": false,  
"siblingSupport": true,  
"printBarcodeAccept": true,  
"lateDelivery": false,  
"addressChange": false,  
"portalCompany": false,  
"recallFlag": false,  
"companiesNeedCleaning": false,  
"deliveryPostpone": true  
}  
},  
"transactions": \[  
{  
"deliveryStatus": "COLLECTED",  
"operationStatus": "IN_DELIVERY_TRANSIT",  
"transactionType": "TAKE_CUSTODY",  
"transactionDateTime": "2023-02-14T18:00:35+03:00",  
"location": "Dağıtım Aracında",  
"transaction": "Gönderi Alındı"  
},  
{  
"deliveryStatus": "TRANSFERRING_COLLECT",  
"operationStatus": "IN_RING_TRANSIT",  
"transactionType": "APPROVE_RING_DEPART",  
"transactionDateTime": "2023-02-14T18:00:38+03:00",  
"location": "Transfer Aracında",  
"transaction": "Transfer merkezine gönderiliyor"  
},  
{  
"deliveryStatus": "TRANSFERRING_COLLECT",  
"operationStatus": "IN_RING_VEHICLE",  
"transactionType": "APPROVE_RING_ARRIVE",  
"transactionDateTime": "2023-02-14T18:00:41+03:00",  
"location": "Transfer Aracında",  
"transaction": "Transfer merkezine ulaştı"  
},  
{  
"deliveryStatus": "READY",  
"operationStatus": "IN_XDOCK",  
"transactionType": "UNLOAD_RING_VEHICLE",  
"transactionDateTime": "2023-02-14T18:00:51+03:00",  
"location": "Adana Tm Aktarma Merkezi",  
"transaction": "Teslimat Şubesine Gönderiliyor"  
},  
{  
"deliveryStatus": "WAITING_FOR_DISPATCH",  
"operationStatus": "IN_XDOCK",  
"transactionType": "UNLOAD_RING_VEHICLE",  
"transactionDateTime": "2023-02-14T18:00:58+03:00",  
"location": "Osmaniye Dağıtım Merkezi",  
"transaction": "Teslimat birimine ulaştı"  
},  
{  
"deliveryStatus": "DELIVERING",  
"operationStatus": "IN_DELIVERY_TRANSIT",  
"transactionType": "APPROVE_COURIER_DEPART",  
"transactionDateTime": "2023-02-14T18:01:07+03:00",  
"location": "Dağıtım Aracında",  
"transaction": "Dağıtıma çıktı"  
},  
{  
"deliveryStatus": "DELIVERED",  
"operationStatus": "IN_RECIPIENT",  
"transactionType": "DELIVERY_ATTEMPT",  
"transactionDateTime": "2023-02-14T18:01:09+03:00",  
"location": "Alıcıda",  
"nonDeliveryReason": "RECIPIENT_NOT_FOUND",  
"deliveredPerson": "Mehmet K**\* (Kendisi)",****"transaction": "Teslim edildi - Mehmet K_\*\\_ (Kendisi)"  
}  
\],  
"productName": "Standart Teslimat",  
"deliveryDatePromised": "2023-02-18T23:59:59+03:00",  
"addressChanged": false,  
"canAddressChange": false,  
"liveTrackingInfoDTO": {  
"temporaryPause": false  
}  
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
    "/rest/deliveryTransaction/getDeliveryTracking": {
      "post": {
        "tags": [
          "Hepsijet > Tracking Servisleri"
        ],
        "summary": "/deliveryTransaction/getDeliveryTracking",
        "description": "sendDeliveryOrderEnhanced ve sendDeliveryOrder serviskerinde payload içerisinde iletilen customerDeliveryNo (Barcode) ile getDeliveryTracking servisinden sorgulama yapılarak gönderiye ait hareketler sorgulama yapılabilir.\n\nToplu sorgulama da yapılabilir. Bir limit bulunmamaktadır.\n\nTek tek sorgulama;\n\n{ \"deliveries\" : \\[\n\n{\n\n\"customerDeliveryNo\" : \"HJT1000000001\"\n\n}\n\n\\]}\n\nToplu Sorgulama;\n\n{ \"deliveries\" : \\[\n\n{\n\n\"customerDeliveryNo\" : \"HJT1000000001\",\n\n},\n\n{\n\n\"customerDeliveryNo\" : \"HJT1000000002\"\n\n}\n\n\\]\n\n}\n\nSorgulama Sonucu;\n\n{  \n\"status\": \"OK\",  \n\"data\": \\[  \n{  \n\"arrivalBranch\": \"Osmaniye Dağıtım Merkezi\",  \n\"customerDeliveryNo\": \"HJT1000000001\",  \n\"barcode\": \"HJT1000000001\",  \n\"deliveryType\": \"RETAIL\",  \n\"currentLocation\": \"Alıcıda\",  \n\"lastTransaction\": \"Teslim edildi - Mehmet K**\\* (Kendisi)\",****\"receiver\": \"Mehmet K**\\*\",****\"sender\": \"YASIN_AKCA\",****\"recipientName\": \"Mehmet Kürşad T**\",**\"deliveryDateCurrent\": \"2023-02-14\",**\"deliverySlotCurrent\": 1,****\"deliveredPerson\": \"Mehmet K**\\*\",  \n\"deliveredDate\": \"2023-02-14T18:01:09+03:00\",  \n\"deliverdPersonIdNo\": \"99999999990\",  \n\"deliveryStatus\": \"DELIVERED\",  \n\"operationStatus\": \"IN_RECIPIENT\",  \n\"transactionType\": \"DELIVERY_ATTEMPT\",  \n\"company\": {  \n\"id\": 7618701,  \n\"name\": \"YASIN_AKCA\",  \n\"companyType\": \"CUSTOMER\",  \n\"abbreviationCode\": \"YASIN_AKCA\",  \n\"corporateName\": \"YASİN AKÇA\",  \n\"companyExtension\": {  \n\"id\": 388596,  \n\"ringDepartureEmailNeeded\": false,  \n\"reportEmail\": false,  \n\"printBarcode\": false,  \n\"companyFlag\": false,  \n\"companyAlternativeReceiverFlag\": false,  \n\"siblingSupport\": true,  \n\"printBarcodeAccept\": true,  \n\"lateDelivery\": false,  \n\"addressChange\": false,  \n\"portalCompany\": false,  \n\"recallFlag\": false,  \n\"companiesNeedCleaning\": false,  \n\"deliveryPostpone\": true  \n}  \n},  \n\"transactions\": \\[  \n{  \n\"deliveryStatus\": \"COLLECTED\",  \n\"operationStatus\": \"IN_DELIVERY_TRANSIT\",  \n\"transactionType\": \"TAKE_CUSTODY\",  \n\"transactionDateTime\": \"2023-02-14T18:00:35+03:00\",  \n\"location\": \"Dağıtım Aracında\",  \n\"transaction\": \"Gönderi Alındı\"  \n},  \n{  \n\"deliveryStatus\": \"TRANSFERRING_COLLECT\",  \n\"operationStatus\": \"IN_RING_TRANSIT\",  \n\"transactionType\": \"APPROVE_RING_DEPART\",  \n\"transactionDateTime\": \"2023-02-14T18:00:38+03:00\",  \n\"location\": \"Transfer Aracında\",  \n\"transaction\": \"Transfer merkezine gönderiliyor\"  \n},  \n{  \n\"deliveryStatus\": \"TRANSFERRING_COLLECT\",  \n\"operationStatus\": \"IN_RING_VEHICLE\",  \n\"transactionType\": \"APPROVE_RING_ARRIVE\",  \n\"transactionDateTime\": \"2023-02-14T18:00:41+03:00\",  \n\"location\": \"Transfer Aracında\",  \n\"transaction\": \"Transfer merkezine ulaştı\"  \n},  \n{  \n\"deliveryStatus\": \"READY\",  \n\"operationStatus\": \"IN_XDOCK\",  \n\"transactionType\": \"UNLOAD_RING_VEHICLE\",  \n\"transactionDateTime\": \"2023-02-14T18:00:51+03:00\",  \n\"location\": \"Adana Tm Aktarma Merkezi\",  \n\"transaction\": \"Teslimat Şubesine Gönderiliyor\"  \n},  \n{  \n\"deliveryStatus\": \"WAITING_FOR_DISPATCH\",  \n\"operationStatus\": \"IN_XDOCK\",  \n\"transactionType\": \"UNLOAD_RING_VEHICLE\",  \n\"transactionDateTime\": \"2023-02-14T18:00:58+03:00\",  \n\"location\": \"Osmaniye Dağıtım Merkezi\",  \n\"transaction\": \"Teslimat birimine ulaştı\"  \n},  \n{  \n\"deliveryStatus\": \"DELIVERING\",  \n\"operationStatus\": \"IN_DELIVERY_TRANSIT\",  \n\"transactionType\": \"APPROVE_COURIER_DEPART\",  \n\"transactionDateTime\": \"2023-02-14T18:01:07+03:00\",  \n\"location\": \"Dağıtım Aracında\",  \n\"transaction\": \"Dağıtıma çıktı\"  \n},  \n{  \n\"deliveryStatus\": \"DELIVERED\",  \n\"operationStatus\": \"IN_RECIPIENT\",  \n\"transactionType\": \"DELIVERY_ATTEMPT\",  \n\"transactionDateTime\": \"2023-02-14T18:01:09+03:00\",  \n\"location\": \"Alıcıda\",  \n\"nonDeliveryReason\": \"RECIPIENT_NOT_FOUND\",  \n\"deliveredPerson\": \"Mehmet K**\\* (Kendisi)\",****\"transaction\": \"Teslim edildi - Mehmet K_\\*\\\\_ (Kendisi)\"  \n}  \n\\],  \n\"productName\": \"Standart Teslimat\",  \n\"deliveryDatePromised\": \"2023-02-18T23:59:59+03:00\",  \n\"addressChanged\": false,  \n\"canAddressChange\": false,  \n\"liveTrackingInfoDTO\": {  \n\"temporaryPause\": false  \n}  \n}  \n\\]  \n}",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{\\r\\n  \\\"deliveries\\\" : [\\r\\n    {\\r\\n      \\\"customerDeliveryNo\\\" : \\\"FPR1000000000\\\"\\r\\n    },\\r\\n     {\\r\\n      \\\"customerDeliveryNo\\\" : \\\"FPR1000000000\\\"\\r\\n    }\\r\\n  ]\\r\\n}\""
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