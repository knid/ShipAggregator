# delivery/findAvailableDeliveryDatesV2

Randevulu iade hizmetinde iade alımı uygun günleri için bu servisi tetikleyerek iade alınacağı lokasyona uygun günlerin listelendiği servistir. Bu servisten dönen uygun tarihlere göre RETURNED tipli gönderi sendDeliveryOrderEnhanced servisinden oluşturulabilir.

/rest/delivery/findAvailableDeliveryDatesV2?startDate=2023-02-21&endDate=2023-02-28&deliveryType=RETURNED&city=İstanbul&town=Kartal

Response Sonucu;

{  
"status": "OK",  
"data": \[  
{  
"cityName": "İstanbul",  
"towns": \[  
{  
"townName": "KARTAL",  
"xDock": \[  
{  
"xDockName": "KARTAL",  
"days": \[  
{  
"date": "2023-02-21",  
"returnedLimit": 1000000  
},  
{  
"date": "2023-02-22",  
"returnedLimit": 1000000  
},  
{  
"date": "2023-02-23",  
"returnedLimit": 1000000  
}  
\]  
}  
\]  
}  
\]  
}  
\]  
}

sendDeliveryOrderEnhanced RETURNED iade gönderimi

{  
"company": {  
"name": "Hepsijet", Bu alan sabittir, bu şekilde gönderi oluşturulması gerekir.  
"abbreviationCode": "Hepsijet" Bu alan sabittir, bu şekilde gönderi oluşturulması gerekir.  
},  
"delivery": {  
"customerDeliveryNo": "HJET100000000", Her gönderi için unique olmalıdır.  
"customerOrderId": "HJET100000000", "customerDeliveryNo" ile aynı gönderilebilir.  
"totalParcels": "1", Sipariş kaç paketten oluşuyorsa, bu alana o girilmelidir.  
"desi": "4", Paketlerin desi ölçüsünü verir.  
"deliverySlotOriginal": "0", Standart gönderilerde bu alan sıfır olarak gönderilir.  
"deliveryDateOriginal": "2023-02-13", Gönderi oluşturulan tarih bilgisi gönderilmelidir.  
"deliveryType": "RETURNED", Randevulu İade süreci işlenecek olan gönderilerde bu alan "RETURNED" değeri ile gönderilmelidir.  
"product": {  
"productCode": "HX_STD" Bu alan standart gönderiler için bu kod ile iletilmelidir.  
},  
"receiver": {  
"companyCustomerId": "HJET100000000", Her yeni adres için benzersiz olmalıdır. recipientAddress alanındaki "companyAddressId" ile aynı gönderilebilir.  
Adres hatalarını önlemek adına başına firma kodu eklenmesi tavsiye edilir. Örnek: HJET100000000  
"firstName": "Hepsijet",  
"lastName": "",  
"phone1": "5321112233",  
"email": ""  
},  
"senderAddress": { Randevulu İade sürecine sahip gönderiler için servis çağırılırken senderAddress ve recipientAddress blokları firma/müşteri bazında yer değiştirir.  
"companyAddressId": "HJET100000000", Her yeni adres için benzersiz olmalıdır. receiver alanındaki "companyCustomerId" ile aynı gönderilebilir.  
Adres hatalarını önlemek adına başına firma kodu eklenmesi tavsiye edilir. Örnek: HJET100000000  
"country": {  
"name": "Türkiye"  
},  
"city": {  
"name": "Osmaniye"  
},  
"town": {  
"name": "Merkez"  
},  
"district": {  
"name": "Hacı Osmanlı"  
},  
"addressLine1": "Hacı Osmanlı Mah. 4005 Sk. No:13 D:3 Merkez / Osmaniye"  
},  
"recipientAddress": { Randevulu İade sürecine sahip gönderiler için servis çağırılırken senderAddress ve recipientAddress blokları içerisindeki değerler firma/müşteri bazında yer değiştirir.  
"companyAddressId": "Hepsijet_ankara_address", Bu alan sabittir, bu şekilde gönderilmelidir.  
"country": {  
"name": "Türkiye"  
},  
"city": {  
"name": "Ankara"  
},  
"town": {  
"name": "Yenimahalle"  
},  
"district": {  
"name": "Demetlale"  
},  
"addressLine1": "Demetlale Mahallesi 405. Cd. No:70 06200 Yenimahalle/Ankara"  
},  
"recipientPerson": "Mehmet Tufan",  
"recipientPersonPhone1": "5322103019"  
},  
"currentXDock": {  
"abbreviationCode": "Hepsijet_ankara" Bu alan sabittir, bu şekilde gönderilmelidir.  
}  
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
      "name": "Hepsijet"
    }
  ],
  "paths": {
    "/rest/delivery/findAvailableDeliveryDatesV2": {
      "get": {
        "tags": [
          "Hepsijet"
        ],
        "summary": "delivery/findAvailableDeliveryDatesV2",
        "description": "Randevulu iade hizmetinde iade alımı uygun günleri için bu servisi tetikleyerek iade alınacağı lokasyona uygun günlerin listelendiği servistir. Bu servisten dönen uygun tarihlere göre RETURNED tipli gönderi sendDeliveryOrderEnhanced servisinden oluşturulabilir.\n\n/rest/delivery/findAvailableDeliveryDatesV2?startDate=2023-02-21&endDate=2023-02-28&deliveryType=RETURNED&city=İstanbul&town=Kartal\n\nResponse Sonucu;\n\n{  \n\"status\": \"OK\",  \n\"data\": \\[  \n{  \n\"cityName\": \"İstanbul\",  \n\"towns\": \\[  \n{  \n\"townName\": \"KARTAL\",  \n\"xDock\": \\[  \n{  \n\"xDockName\": \"KARTAL\",  \n\"days\": \\[  \n{  \n\"date\": \"2023-02-21\",  \n\"returnedLimit\": 1000000  \n},  \n{  \n\"date\": \"2023-02-22\",  \n\"returnedLimit\": 1000000  \n},  \n{  \n\"date\": \"2023-02-23\",  \n\"returnedLimit\": 1000000  \n}  \n\\]  \n}  \n\\]  \n}  \n\\]  \n}  \n\\]  \n}\n\nsendDeliveryOrderEnhanced RETURNED iade gönderimi\n\n{  \n\"company\": {  \n\"name\": \"Hepsijet\", Bu alan sabittir, bu şekilde gönderi oluşturulması gerekir.  \n\"abbreviationCode\": \"Hepsijet\" Bu alan sabittir, bu şekilde gönderi oluşturulması gerekir.  \n},  \n\"delivery\": {  \n\"customerDeliveryNo\": \"HJET100000000\", Her gönderi için unique olmalıdır.  \n\"customerOrderId\": \"HJET100000000\", \"customerDeliveryNo\" ile aynı gönderilebilir.  \n\"totalParcels\": \"1\", Sipariş kaç paketten oluşuyorsa, bu alana o girilmelidir.  \n\"desi\": \"4\", Paketlerin desi ölçüsünü verir.  \n\"deliverySlotOriginal\": \"0\", Standart gönderilerde bu alan sıfır olarak gönderilir.  \n\"deliveryDateOriginal\": \"2023-02-13\", Gönderi oluşturulan tarih bilgisi gönderilmelidir.  \n\"deliveryType\": \"RETURNED\", Randevulu İade süreci işlenecek olan gönderilerde bu alan \"RETURNED\" değeri ile gönderilmelidir.  \n\"product\": {  \n\"productCode\": \"HX_STD\" Bu alan standart gönderiler için bu kod ile iletilmelidir.  \n},  \n\"receiver\": {  \n\"companyCustomerId\": \"HJET100000000\", Her yeni adres için benzersiz olmalıdır. recipientAddress alanındaki \"companyAddressId\" ile aynı gönderilebilir.  \nAdres hatalarını önlemek adına başına firma kodu eklenmesi tavsiye edilir. Örnek: HJET100000000  \n\"firstName\": \"Hepsijet\",  \n\"lastName\": \"\",  \n\"phone1\": \"5321112233\",  \n\"email\": \"\"  \n},  \n\"senderAddress\": { Randevulu İade sürecine sahip gönderiler için servis çağırılırken senderAddress ve recipientAddress blokları firma/müşteri bazında yer değiştirir.  \n\"companyAddressId\": \"HJET100000000\", Her yeni adres için benzersiz olmalıdır. receiver alanındaki \"companyCustomerId\" ile aynı gönderilebilir.  \nAdres hatalarını önlemek adına başına firma kodu eklenmesi tavsiye edilir. Örnek: HJET100000000  \n\"country\": {  \n\"name\": \"Türkiye\"  \n},  \n\"city\": {  \n\"name\": \"Osmaniye\"  \n},  \n\"town\": {  \n\"name\": \"Merkez\"  \n},  \n\"district\": {  \n\"name\": \"Hacı Osmanlı\"  \n},  \n\"addressLine1\": \"Hacı Osmanlı Mah. 4005 Sk. No:13 D:3 Merkez / Osmaniye\"  \n},  \n\"recipientAddress\": { Randevulu İade sürecine sahip gönderiler için servis çağırılırken senderAddress ve recipientAddress blokları içerisindeki değerler firma/müşteri bazında yer değiştirir.  \n\"companyAddressId\": \"Hepsijet_ankara_address\", Bu alan sabittir, bu şekilde gönderilmelidir.  \n\"country\": {  \n\"name\": \"Türkiye\"  \n},  \n\"city\": {  \n\"name\": \"Ankara\"  \n},  \n\"town\": {  \n\"name\": \"Yenimahalle\"  \n},  \n\"district\": {  \n\"name\": \"Demetlale\"  \n},  \n\"addressLine1\": \"Demetlale Mahallesi 405. Cd. No:70 06200 Yenimahalle/Ankara\"  \n},  \n\"recipientPerson\": \"Mehmet Tufan\",  \n\"recipientPersonPhone1\": \"5322103019\"  \n},  \n\"currentXDock\": {  \n\"abbreviationCode\": \"Hepsijet_ankara\" Bu alan sabittir, bu şekilde gönderilmelidir.  \n}  \n}",
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
            "name": "startDate",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "example": "2023-02-21"
          },
          {
            "name": "endDate",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "example": "2023-02-28"
          },
          {
            "name": "deliveryType",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "example": "RETURNED"
          },
          {
            "name": "city",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "example": "İstanbul"
          },
          {
            "name": "town",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "example": "Kartal"
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