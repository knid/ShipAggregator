# /sendDeliveryAdvance/v2

MERCHANT KAYDI OLUŞTURMA  
advance/sendDeliveryAdvance/v2  
Merchant kaydı oluştururken ilgili firma için tek bir kez kayıt gönderilmelidir.

{  
"orderId":"HJETarBilgisayarDepo3", Merchant kaydı oluştururken Depo adı bilgisi verilebilir.

"orderDate":"2023-01-17", Kayıt esnasında kayıt tarihi bilgisi girilmelidir.  
"receiver":{  
"companyCustomerId":"HJET123456789", Kayıt esnasında İstenilen bir değer girilebilir.  
"firstName":"Mehmet", Kayıt esnasında İstenilen bir değer girilebilir.  
"lastName":"Tufan", Kayıt esnasında İstenilen bir değer girilebilir.  
"phone1":"5322221133", Kayıt esnasında İstenilen bir değer girilebilir.  
"email":"[bykursadtufan@gmail.com](mailto:bykursadtufan@gmail.com)" Kayıt esnasında İstenilen bir değer girilebilir.  
},  
"deliveries":\[  
{  
"senderCompany":{  
"companyCompanyId":1, Kayıt esnasında değer 1 olarak gelebilir.  
"name":"HJET_Firmasi", Gönderim yapılacak firma bilgisinin adı yazılmalıdır. Zorunlu Alan  
"abbreviationCode":"HJET_Firmasi", Gönderim yapılacak Company Name bilgisiyle aynı ad ya da firma kısa kod olarak da yazılabilir.. Zorunlu Alan

```
        "accountId":"HJETarBilgisayarDepo3",   Merchant kaydı oluştururken orderid’deki bilgi yazılabilir.
        "contactEmails": [ 
                " yazilim@HJETarbilgisayar.com"   Kayıt esnasında İstenilen bir değer girilebilir.
            ]

```

},  
"deliveryType":"MARKET_PLACE", Zorunlu Alandır.  
"deliverySlot":"0", Kayıt esnasında değer “0” olarak girilmelidir.  
"desi":"1", Kayıt esnasında İstenilen bir değer girilebilir.  
"product":{  
"productCode":"HX_STD" Zorunlu Alandır.  
},  
"senderAddress":{  
"companyAddressId":"HJETarBilgisayarDepo3", Firma Adres bilgisidir. Zorunlu Alandır.  
"country":{  
"name":"Türkiye" Firma Adres bilgisidir. Zorunlu Alandır.  
},  
"city":{  
"name":"İstanbul" Firma Adres bilgisidir. Zorunlu Alandır.  
},  
"town":{  
"name":"Kartal" Firma Adres bilgisidir. Zorunlu Alandır.  
},  
"district": {  
"name": "Esentepe" Firma Adres bilgisidir. Zorunlu Alandır.  
},  
"addressLine1":"Esentepe, Yenice Sk. No:35 K:1 34870 Kartal/İstanbul" Firma Adres bilgisidir. Zorunlu Alandır.  
},  
"recipientAddress":{  
"companyAddressId":"HJET123456789", Kayıt esnasında İstenilen bir değer girilebilir.  
"country":{  
"name":"Türkiye" Kayıt esnasında İstenilen bir değer girilebilir.  
},  
"city":{  
"name":"İstanbul" Kayıt esnasında İstenilen bir değer girilebilir.  
},  
"town":{  
"name":"Kartal" Kayıt esnasında İstenilen bir değer girilebilir.  
},  
"district":{  
"name":"Esentepe" Kayıt esnasında İstenilen bir değer girilebilir.  
},  
"addressLine1":"Petrol İş Mah. Kamelya Sk. No:10 K:1 34870 Kartal/İstanbul" Kayıt esnasında İstenilen bir değer girilebilir.  
},

```
     "applicationNo":""  Kayıt esnasında boş gönderilebilir.
  }

```

\]  
}

# OpenAPI definition

````json
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
    "/advance/sendDeliveryAdvance/v2": {
      "post": {
        "tags": [
          "Hepsijet > Gonderi Servisleri"
        ],
        "summary": "/sendDeliveryAdvance/v2",
        "description": "MERCHANT KAYDI OLUŞTURMA  \nadvance/sendDeliveryAdvance/v2  \nMerchant kaydı oluştururken ilgili firma için tek bir kez kayıt gönderilmelidir.\n\n{  \n\"orderId\":\"HJETarBilgisayarDepo3\", Merchant kaydı oluştururken Depo adı bilgisi verilebilir.\n\n\"orderDate\":\"2023-01-17\", Kayıt esnasında kayıt tarihi bilgisi girilmelidir.  \n\"receiver\":{  \n\"companyCustomerId\":\"HJET123456789\", Kayıt esnasında İstenilen bir değer girilebilir.  \n\"firstName\":\"Mehmet\", Kayıt esnasında İstenilen bir değer girilebilir.  \n\"lastName\":\"Tufan\", Kayıt esnasında İstenilen bir değer girilebilir.  \n\"phone1\":\"5322221133\", Kayıt esnasında İstenilen bir değer girilebilir.  \n\"email\":\"[bykursadtufan@gmail.com](mailto:bykursadtufan@gmail.com)\" Kayıt esnasında İstenilen bir değer girilebilir.  \n},  \n\"deliveries\":\\[  \n{  \n\"senderCompany\":{  \n\"companyCompanyId\":1, Kayıt esnasında değer 1 olarak gelebilir.  \n\"name\":\"HJET_Firmasi\", Gönderim yapılacak firma bilgisinin adı yazılmalıdır. Zorunlu Alan  \n\"abbreviationCode\":\"HJET_Firmasi\", Gönderim yapılacak Company Name bilgisiyle aynı ad ya da firma kısa kod olarak da yazılabilir.. Zorunlu Alan\n\n```\n        \"accountId\":\"HJETarBilgisayarDepo3\",   Merchant kaydı oluştururken orderid’deki bilgi yazılabilir.\n        \"contactEmails\": [ \n                \" yazilim@HJETarbilgisayar.com\"   Kayıt esnasında İstenilen bir değer girilebilir.\n            ]\n\n```\n\n},  \n\"deliveryType\":\"MARKET_PLACE\", Zorunlu Alandır.  \n\"deliverySlot\":\"0\", Kayıt esnasında değer “0” olarak girilmelidir.  \n\"desi\":\"1\", Kayıt esnasında İstenilen bir değer girilebilir.  \n\"product\":{  \n\"productCode\":\"HX_STD\" Zorunlu Alandır.  \n},  \n\"senderAddress\":{  \n\"companyAddressId\":\"HJETarBilgisayarDepo3\", Firma Adres bilgisidir. Zorunlu Alandır.  \n\"country\":{  \n\"name\":\"Türkiye\" Firma Adres bilgisidir. Zorunlu Alandır.  \n},  \n\"city\":{  \n\"name\":\"İstanbul\" Firma Adres bilgisidir. Zorunlu Alandır.  \n},  \n\"town\":{  \n\"name\":\"Kartal\" Firma Adres bilgisidir. Zorunlu Alandır.  \n},  \n\"district\": {  \n\"name\": \"Esentepe\" Firma Adres bilgisidir. Zorunlu Alandır.  \n},  \n\"addressLine1\":\"Esentepe, Yenice Sk. No:35 K:1 34870 Kartal/İstanbul\" Firma Adres bilgisidir. Zorunlu Alandır.  \n},  \n\"recipientAddress\":{  \n\"companyAddressId\":\"HJET123456789\", Kayıt esnasında İstenilen bir değer girilebilir.  \n\"country\":{  \n\"name\":\"Türkiye\" Kayıt esnasında İstenilen bir değer girilebilir.  \n},  \n\"city\":{  \n\"name\":\"İstanbul\" Kayıt esnasında İstenilen bir değer girilebilir.  \n},  \n\"town\":{  \n\"name\":\"Kartal\" Kayıt esnasında İstenilen bir değer girilebilir.  \n},  \n\"district\":{  \n\"name\":\"Esentepe\" Kayıt esnasında İstenilen bir değer girilebilir.  \n},  \n\"addressLine1\":\"Petrol İş Mah. Kamelya Sk. No:10 K:1 34870 Kartal/İstanbul\" Kayıt esnasında İstenilen bir değer girilebilir.  \n},\n\n```\n     \"applicationNo\":\"\"  Kayıt esnasında boş gönderilebilir.\n  }\n\n```\n\n\\]  \n}",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{\\r\\n    \\\"company\\\": {\\r\\n        \\\"name\\\": \\\"FAPRIKA\\\",\\r\\n        \\\"abbreviationCode\\\": \\\"FPRKA\\\"\\r\\n    },\\r\\n    \\\"delivery\\\": {\\r\\n        \\\"customerDeliveryNo\\\": \\\"FPR12200000000\\\",\\r\\n        \\\"customerOrderId\\\": \\\"FPR12200000000\\\",\\r\\n        \\\"totalParcels\\\": \\\"1\\\",\\r\\n        \\\"desi\\\": \\\"8\\\", \\r\\n        \\\"deliverySlotOriginal\\\": \\\"0\\\",\\r\\n        \\\"deliveryDateOriginal\\\": \\\"2022-12-06\\\", \\r\\n        \\\"deliveryType\\\": \\\"RETAIL\\\",\\r\\n        \\\"product\\\": {\\r\\n            \\\"productCode\\\": \\\"HX_STD\\\" \\r\\n        },\\r\\n        \\\"receiver\\\": {\\r\\n            \\\"companyCustomerId\\\": \\\"FPR12200000000\\\", \\r\\n            \\\"firstName\\\": \\\"Mehmet Kürşad\\\",\\r\\n            \\\"lastName\\\": \\\"Tufan\\\",\\r\\n            \\\"phone1\\\": \\\"5494380768\\\",\\r\\n            \\\"email\\\": \\\"mehmet.tufan@hepsijet.com\\\"\\r\\n        },\\r\\n        \\\"senderAddress\\\": {\\r\\n            \\\"companyAddressId\\\": \\\"faprika_ankra_address\\\",\\r\\n            \\\"country\\\": {\\r\\n                \\\"name\\\": \\\"Türkiye\\\"\\r\\n            },\\r\\n            \\\"city\\\": {\\r\\n                \\\"name\\\": \\\"Ankara\\\"\\r\\n            },\\r\\n            \\\"town\\\": {\\r\\n                \\\"name\\\": \\\"Yenimahalle\\\"\\r\\n            },\\r\\n            \\\"district\\\": {\\r\\n                \\\"name\\\": \\\"Demetlale\\\"\\r\\n            },\\r\\n            \\\"addressLine1\\\": \\\"Demetlale Mahallesi 405. Cd. No:70 06200 Yenimahalle/Ankara\\\"\\r\\n        },\\r\\n        \\\"recipientAddress\\\": {\\r\\n            \\\"companyAddressId\\\": \\\"FPR12200000000\\\",\\r\\n            \\\"country\\\": {\\r\\n                \\\"name\\\": \\\"Türkiye\\\"\\r\\n            },\\r\\n            \\\"city\\\": {\\r\\n                \\\"name\\\": \\\"Osmaniye\\\"\\r\\n            },\\r\\n            \\\"town\\\": {\\r\\n                \\\"name\\\": \\\"Merkez\\\"\\r\\n            },\\r\\n            \\\"district\\\": {\\r\\n                \\\"name\\\": \\\"Hacı Osmanlı\\\"\\r\\n            },\\r\\n            \\\"addressLine1\\\": \\\"Hacı Osmanlı Mah. 4005 Sk. No:13 D:3 Merkez / Osmaniye\\\"\\r\\n        },\\r\\n        \\\"recipientPerson\\\": \\\"Mehmet Kürşad Tufan\\\",\\r\\n        \\\"recipientPersonPhone1\\\": \\\"5494380768\\\"\\r\\n    },\\r\\n    \\\"currentXDock\\\": {\\r\\n        \\\"abbreviationCode\\\": \\\"FPRKA_ANKRA\\\"\\r\\n    }\\r\\n}\""
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
````