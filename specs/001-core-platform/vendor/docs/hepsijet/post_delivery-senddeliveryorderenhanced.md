# /sendDeliveryOrderEnhanced

sendDeliveryOrderEnhanced servisi fiziksel gönderilerin Hepsijet'e ileten servistir.

Bu servis kapsamında hangi parametrelerin sabit, hangilerin değişken iletilmesi konusunda bilgi verilecektir.

Company bloğundaki "name" ve "abbreviationCode" bilgileri hepsijet tarafından belirlenen parametrelerdir. Bu bilgiler hepsijet tarafından iletilmektedir. Bu alanlar **sabittir** farklı bilgi girilerek iletimeMEsi gerekiyor.

"company": {  
"name": "Hepsijet_Firmasi",  
"abbreviationCode": "Hpsjet_Frmsi"  
},

delivery bloğu;

"customerDeliveryNo" bilgisi 9-16 karakter arası unique sağlamak adına Firma gönderilerin başına firma kısa kodu eklenerek unique gönderim sağlanabilir.

Örnek: HJT1234567890123 olarak gönderilebilir.

"customerOrderId" değeri customerDeliveryNo aynı gönderilebilir.

"customerOrderId": "HJT1234567890123"

totalparcels;

Sipariş kaç paketten oluşuyorsa, bu alana o değer girilmelidir.

"totalParcels": "4",

Desi;

Paketin desi ölçüsünü verir. Bu alana desi miktarı girilerek iletilmelidir. delivery-update servisi ile paket teslim edilene kadar güncellenebilir.

Not: Desi güncellenirken totalparcel değeri gönderilmesi zorunludur.

"desi": "9"

deliveryDateOriginal;

"2023-01-10" Gönderi oluşturulan tarih bilgisi gönderilmelidir.

deliverySlotOriginal;

Standart Teslimat HX_STD gönderilerde bu alan "0" sıfır olarak gönderilir. Aynı ve ertesi gün gönderimlerde slot’un firma tarafında Hepsijete bildirmesi bekleniyor.

Bu gönderi firmadan firmaya değişen bir değer değildir.

Bunlar;  
1 sabah 09:00 dan 13:00 kadar  
2 sabah 13:00 dan 18:00 kadar  
3 ise sabah 18:00 dan 23:00 kadar olan zaman aralığı temsil etmektedir.

"deliveryType"

\> PERAKENDE = RETAIL

\> PAZAR YERİ = MARKET_PLACE

\> İADE RANDEVULU = RETURNED

Product Code;

Standart Teslimat HX_STD

Aynı Gün Teslimat HX_SD

Ertesi Gün Teslimat HX_ND

Returned İade teslimat HX_STD

Delivery ve ProductCode statülerine göre gönderim tipleri;

Standart Teslimat

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "0",  
"product": {  
"productCode": "HX_STD"

Bugün Teslimat

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "1",  
"product": {  
"productCode": "HX_SD"

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "2",  
"product": {  
"productCode": "HX_SD"

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "3",  
"product": {  
"productCode": "HX_SD" -- Bugün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.

Ertesi Gün Teslimat

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "1",  
"product": {  
"productCode": "HX_ND"

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "2",  
"product": {  
"productCode": "HX_ND"

"deliveryType": "RETAIL",  
"deliverySlotOriginal": "3",  
"product": {  
"productCode": "HX_ND" -- Ertesi gün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.

PAZAR YERİ MARKET_PLACE

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "0",  
"product": {  
"productCode": "HX_STD"

Bugün Teslimat

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "1",  
"product": {  
"productCode": "HX_SD"

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "2",  
"product": {  
"productCode": "HX_SD"

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "3",  
"product": {  
"productCode": "HX_SD" -- Bugün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.

Ertesi Gün Teslimat

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "1",  
"product": {  
"productCode": "HX_ND"

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "2",  
"product": {  
"productCode": "HX_ND"

"deliveryType": "MARKET_PLACE",  
"deliverySlotOriginal": "3",  
"product": {  
"productCode": "HX_ND" -- Ertesi gün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.

\> İADE RANDEVULU = RETURNED

"deliveryType": "RETURNED",  
"deliverySlotOriginal": "0",  
"product": {  
"productCode": "HX_STD" İade süreçlerinde productCode bilgisi HX_STD olarak iletilmelidir.

SenderAddress Bloğu;

Bu alanda firma tarafında iletilen adres bilgileri sisteme tanımlandıktan sonra birebir aynı olarak iletilmesi bekleniyor. sender, companyAddressId bilgisi hepsijet tarafından iletilen sabit bir bilgidir.

"senderAddress": {  
"companyAddressId": "hepsijetaddress",  
"country": {  
"name": "Türkiye"  
},  
"city": {  
"name": "İstanbul"  
},  
"town": {  
"name": "Kartal"  
},  
"district": {  
"name": "Petrol İş"  
},  
"addressLine1": "Petrol İş Mah. Kamelya Sk. No:10 D:1 Kartal/İstanbul"  
},

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
    "/delivery/sendDeliveryOrderEnhanced": {
      "post": {
        "tags": [
          "Hepsijet > Gonderi Servisleri"
        ],
        "summary": "/sendDeliveryOrderEnhanced",
        "description": "sendDeliveryOrderEnhanced servisi fiziksel gönderilerin Hepsijet'e ileten servistir.\n\nBu servis kapsamında hangi parametrelerin sabit, hangilerin değişken iletilmesi konusunda bilgi verilecektir.\n\nCompany bloğundaki \"name\" ve \"abbreviationCode\" bilgileri hepsijet tarafından belirlenen parametrelerdir. Bu bilgiler hepsijet tarafından iletilmektedir. Bu alanlar **sabittir** farklı bilgi girilerek iletimeMEsi gerekiyor.\n\n\"company\": {  \n\"name\": \"Hepsijet_Firmasi\",  \n\"abbreviationCode\": \"Hpsjet_Frmsi\"  \n},\n\ndelivery bloğu;\n\n\"customerDeliveryNo\" bilgisi 9-16 karakter arası unique sağlamak adına Firma gönderilerin başına firma kısa kodu eklenerek unique gönderim sağlanabilir.\n\nÖrnek: HJT1234567890123 olarak gönderilebilir.\n\n\"customerOrderId\" değeri customerDeliveryNo aynı gönderilebilir.\n\n\"customerOrderId\": \"HJT1234567890123\"\n\ntotalparcels;\n\nSipariş kaç paketten oluşuyorsa, bu alana o değer girilmelidir.\n\n\"totalParcels\": \"4\",\n\nDesi;\n\nPaketin desi ölçüsünü verir. Bu alana desi miktarı girilerek iletilmelidir. delivery-update servisi ile paket teslim edilene kadar güncellenebilir.\n\nNot: Desi güncellenirken totalparcel değeri gönderilmesi zorunludur.\n\n\"desi\": \"9\"\n\ndeliveryDateOriginal;\n\n\"2023-01-10\" Gönderi oluşturulan tarih bilgisi gönderilmelidir.\n\ndeliverySlotOriginal;\n\nStandart Teslimat HX_STD gönderilerde bu alan \"0\" sıfır olarak gönderilir. Aynı ve ertesi gün gönderimlerde slot’un firma tarafında Hepsijete bildirmesi bekleniyor.\n\nBu gönderi firmadan firmaya değişen bir değer değildir.\n\nBunlar;  \n1 sabah 09:00 dan 13:00 kadar  \n2 sabah 13:00 dan 18:00 kadar  \n3 ise sabah 18:00 dan 23:00 kadar olan zaman aralığı temsil etmektedir.\n\n\"deliveryType\"\n\n\\> PERAKENDE = RETAIL\n\n\\> PAZAR YERİ = MARKET_PLACE\n\n\\> İADE RANDEVULU = RETURNED\n\nProduct Code;\n\nStandart Teslimat HX_STD\n\nAynı Gün Teslimat HX_SD\n\nErtesi Gün Teslimat HX_ND\n\nReturned İade teslimat HX_STD\n\nDelivery ve ProductCode statülerine göre gönderim tipleri;\n\nStandart Teslimat\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"0\",  \n\"product\": {  \n\"productCode\": \"HX_STD\"\n\nBugün Teslimat\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"1\",  \n\"product\": {  \n\"productCode\": \"HX_SD\"\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"2\",  \n\"product\": {  \n\"productCode\": \"HX_SD\"\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"3\",  \n\"product\": {  \n\"productCode\": \"HX_SD\" -- Bugün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.\n\nErtesi Gün Teslimat\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"1\",  \n\"product\": {  \n\"productCode\": \"HX_ND\"\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"2\",  \n\"product\": {  \n\"productCode\": \"HX_ND\"\n\n\"deliveryType\": \"RETAIL\",  \n\"deliverySlotOriginal\": \"3\",  \n\"product\": {  \n\"productCode\": \"HX_ND\" -- Ertesi gün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.\n\nPAZAR YERİ MARKET_PLACE\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"0\",  \n\"product\": {  \n\"productCode\": \"HX_STD\"\n\nBugün Teslimat\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"1\",  \n\"product\": {  \n\"productCode\": \"HX_SD\"\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"2\",  \n\"product\": {  \n\"productCode\": \"HX_SD\"\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"3\",  \n\"product\": {  \n\"productCode\": \"HX_SD\" -- Bugün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.\n\nErtesi Gün Teslimat\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"1\",  \n\"product\": {  \n\"productCode\": \"HX_ND\"\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"2\",  \n\"product\": {  \n\"productCode\": \"HX_ND\"\n\n\"deliveryType\": \"MARKET_PLACE\",  \n\"deliverySlotOriginal\": \"3\",  \n\"product\": {  \n\"productCode\": \"HX_ND\" -- Ertesi gün gönderimlerde deliverySlotOriginal firmanın Hepsijete bildirmesini bekleniyor.\n\n\\> İADE RANDEVULU = RETURNED\n\n\"deliveryType\": \"RETURNED\",  \n\"deliverySlotOriginal\": \"0\",  \n\"product\": {  \n\"productCode\": \"HX_STD\" İade süreçlerinde productCode bilgisi HX_STD olarak iletilmelidir.\n\nSenderAddress Bloğu;\n\nBu alanda firma tarafında iletilen adres bilgileri sisteme tanımlandıktan sonra birebir aynı olarak iletilmesi bekleniyor. sender, companyAddressId bilgisi hepsijet tarafından iletilen sabit bir bilgidir.\n\n\"senderAddress\": {  \n\"companyAddressId\": \"hepsijetaddress\",  \n\"country\": {  \n\"name\": \"Türkiye\"  \n},  \n\"city\": {  \n\"name\": \"İstanbul\"  \n},  \n\"town\": {  \n\"name\": \"Kartal\"  \n},  \n\"district\": {  \n\"name\": \"Petrol İş\"  \n},  \n\"addressLine1\": \"Petrol İş Mah. Kamelya Sk. No:10 D:1 Kartal/İstanbul\"  \n},",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{\\r\\n    \\\"company\\\": {\\r\\n        \\\"name\\\": \\\"FAPRIKA\\\",\\r\\n        \\\"abbreviationCode\\\": \\\"FPRKA\\\"\\r\\n    },\\r\\n    \\\"delivery\\\": {\\r\\n        \\\"customerDeliveryNo\\\": \\\"FPR1000000000\\\",\\r\\n        \\\"customerOrderId\\\": \\\"FPR1000000000\\\",\\r\\n        \\\"totalParcels\\\": \\\"1\\\",\\r\\n        \\\"desi\\\": \\\"8\\\", \\r\\n        \\\"deliverySlotOriginal\\\": \\\"0\\\",\\r\\n        \\\"deliveryDateOriginal\\\": \\\"2022-12-02\\\", \\r\\n        \\\"deliveryType\\\": \\\"RETAIL\\\",\\r\\n        \\\"product\\\": {\\r\\n            \\\"productCode\\\": \\\"HX_STD\\\" \\r\\n        },\\r\\n        \\\"receiver\\\": {\\r\\n            \\\"companyCustomerId\\\": \\\"FPR1000000000\\\", \\r\\n            \\\"firstName\\\": \\\"Mehmet Kürşad\\\",\\r\\n            \\\"lastName\\\": \\\"Tufan\\\",\\r\\n            \\\"phone1\\\": \\\"5494380768\\\",\\r\\n            \\\"email\\\": \\\"mehmet.tufan@hepsijet.com\\\"\\r\\n        },\\r\\n        \\\"senderAddress\\\": {\\r\\n            \\\"companyAddressId\\\": \\\"faprika_ankra_address\\\",\\r\\n            \\\"country\\\": {\\r\\n                \\\"name\\\": \\\"Türkiye\\\"\\r\\n            },\\r\\n            \\\"city\\\": {\\r\\n                \\\"name\\\": \\\"Ankara\\\"\\r\\n            },\\r\\n            \\\"town\\\": {\\r\\n                \\\"name\\\": \\\"Yenimahalle\\\"\\r\\n            },\\r\\n            \\\"district\\\": {\\r\\n                \\\"name\\\": \\\"Demetlale\\\"\\r\\n            },\\r\\n            \\\"addressLine1\\\": \\\"Demetlale Mahallesi 405. Cd. No:70 06200 Yenimahalle/Ankara\\\"\\r\\n        },\\r\\n        \\\"recipientAddress\\\": {\\r\\n            \\\"companyAddressId\\\": \\\"FPR1000000000\\\",\\r\\n            \\\"country\\\": {\\r\\n                \\\"name\\\": \\\"Türkiye\\\"\\r\\n            },\\r\\n            \\\"city\\\": {\\r\\n                \\\"name\\\": \\\"Osmaniye\\\"\\r\\n            },\\r\\n            \\\"town\\\": {\\r\\n                \\\"name\\\": \\\"Merkez\\\"\\r\\n            },\\r\\n            \\\"district\\\": {\\r\\n                \\\"name\\\": \\\"Hacı Osmanlı\\\"\\r\\n            },\\r\\n            \\\"addressLine1\\\": \\\"Hacı Osmanlı Mah. 4005 Sk. No:13 D:3 Merkez / Osmaniye\\\"\\r\\n        },\\r\\n        \\\"recipientPerson\\\": \\\"Mehmet Kürşad Tufan\\\",\\r\\n        \\\"recipientPersonPhone1\\\": \\\"5494380768\\\"\\r\\n    },\\r\\n    \\\"currentXDock\\\": {\\r\\n        \\\"abbreviationCode\\\": \\\"FPRKA_ANKRA\\\"\\r\\n    }\\r\\n}\""
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