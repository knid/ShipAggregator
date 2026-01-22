# /delivery-update

\[if gte mso 9\]>  
  
  
  
  
  
  
  

\[if gte mso 9\]>  
  
Normal  
0  
  
  
21  
  
  
false  
false  
false  
  
TR  
X-NONE  
X-NONE  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  

\[if gte mso 9\]>  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  
  

\[if gte mso 10\]>  
  
  

Aşağıdaki örneklerdeki gibi güncellenmesi istenilen requestler ayrı ayrı gönderilebilir.  
Request Sample1:  
● Desi bilgisi güncellenmek isteniyorsa totalParcel parametresinin de gönderilmesi zorunludur.  
● totalParcel değeri iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.  
● desi değeri gönderi teslim edilene kadar güncellenebilir.  
{  
"deliveryUpdateType": "TOTAL_PARCEL",  
"customerDeliveryNo": "3561217255F59",  
"totalParcelInfo": {  
"totalParcel": 3,  
"desi": \[1, 5, 3\]  
}  
}

Request Sample2:  
● Standart Teslimattan Bugün Teslimata güncellenen bir sipariş için aşağıdaki request ile gelinmesi gerekir.  
● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.

{  
"deliveryUpdateType": "PRODUCT_CODE",  
"customerDeliveryNo": "3561217255F55",  
"productCodeInfo": {  
"productCode": "HX_SD",  
"deliveryDateOriginal": "2020-12-16T17:00:00.0000000+00:00",  
"deliverySlot": 1,  
"deliveryStartTime": "12:00:00",  
"deliveryEndTime": "00:00:00"  
}  
}  
Request Sample3:  
● Standart Teslimat olarak güncellenen bir sipariş için aşağıdaki request ile gelinmesi gerekir.  
● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.

{  
"deliveryUpdateType": "PRODUCT_CODE",  
"customerDeliveryNo": "3561217255F55",  
"productCodeInfo": {  
"productCode": "HX_STD",  
"deliveryDateOriginal": "2020-12-16T17:00:00.0000000+00:00"  
}  
}

Request Sample4:  
● Bugün Teslimattan Yarın Teslimat ya da Yarın Teslimattan Bugün Teslimat olarak güncellenen bir sipariş için aşağıdaki request ile gelinmesi gerekir.  
● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.

{  
"deliveryUpdateType": "PRODUCT_CODE",  
"customerDeliveryNo": "3561217255F55",  
"productCodeInfo": {  
"productCode": "HX_ND"  
}  
}

Request Sample5:  
● Siparişteki alıcıya ait bilgileri güncellemek için aşağıdaki request ile gelinmelidir.  
● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.

{  
"deliveryUpdateType": "COMPANY_CUSTOMER",  
"customerDeliveryNo": "3561217255F55",  
"companyCustomerInfo": {  
"companyCustomerId": "123444",  
"firstName": "ali",  
"lastName": "can",  
"email": "ali@can",  
"gsm1": "0536"  
}  
}

Request Sample6:  
● Siparişteki adres bilgisini güncellemek için aşağıdaki request ile gelinmelidir.  
● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.

{  
"deliveryUpdateType": "COMPANY_ADDRESS",  
"customerDeliveryNo": "3561217255F55",  
"companyAddressInfo": {  
"companyAddressId": "123456",  
"addressLine1": "kuşakçı sok no:1aaa5",  
"countryName": "türkiye",  
"cityName": "istnbulu",  
"townName": "mallltepe",  
"districtName": "adss"  
}  
}

Request Sample7:  
● Siparişteki alıcıya ait bilgileri güncellemek için aşağıdaki request ile gelinmelidir.  
● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.

{  
"deliveryUpdateType": "RECIPIENT_PERSON",  
"customerDeliveryNo": "6271087689840",  
"recipientPersonInfo": {  
"recipientPerson": "ali can",  
"recipientPersonPhone1": "0536",  
"recipientPersonPhone2": "0505",  
"recipientPersonEmail": "ali@can"  
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
      "name": "Hepsijet > Gonderi Servisleri"
    }
  ],
  "paths": {
    "/delivery-update": {
      "post": {
        "tags": [
          "Hepsijet > Gonderi Servisleri"
        ],
        "summary": "/delivery-update",
        "description": "\\[if gte mso 9\\]>  \n  \n  \n  \n  \n  \n  \n  \n\n\\[if gte mso 9\\]>  \n  \nNormal  \n0  \n  \n  \n21  \n  \n  \nfalse  \nfalse  \nfalse  \n  \nTR  \nX-NONE  \nX-NONE  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n\n\\[if gte mso 9\\]>  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n  \n\n\\[if gte mso 10\\]>  \n  \n  \n\nAşağıdaki örneklerdeki gibi güncellenmesi istenilen requestler ayrı ayrı gönderilebilir.  \nRequest Sample1:  \n● Desi bilgisi güncellenmek isteniyorsa totalParcel parametresinin de gönderilmesi zorunludur.  \n● totalParcel değeri iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.  \n● desi değeri gönderi teslim edilene kadar güncellenebilir.  \n{  \n\"deliveryUpdateType\": \"TOTAL_PARCEL\",  \n\"customerDeliveryNo\": \"3561217255F59\",  \n\"totalParcelInfo\": {  \n\"totalParcel\": 3,  \n\"desi\": \\[1, 5, 3\\]  \n}  \n}\n\nRequest Sample2:  \n● Standart Teslimattan Bugün Teslimata güncellenen bir sipariş için aşağıdaki request ile gelinmesi gerekir.  \n● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.\n\n{  \n\"deliveryUpdateType\": \"PRODUCT_CODE\",  \n\"customerDeliveryNo\": \"3561217255F55\",  \n\"productCodeInfo\": {  \n\"productCode\": \"HX_SD\",  \n\"deliveryDateOriginal\": \"2020-12-16T17:00:00.0000000+00:00\",  \n\"deliverySlot\": 1,  \n\"deliveryStartTime\": \"12:00:00\",  \n\"deliveryEndTime\": \"00:00:00\"  \n}  \n}  \nRequest Sample3:  \n● Standart Teslimat olarak güncellenen bir sipariş için aşağıdaki request ile gelinmesi gerekir.  \n● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.\n\n{  \n\"deliveryUpdateType\": \"PRODUCT_CODE\",  \n\"customerDeliveryNo\": \"3561217255F55\",  \n\"productCodeInfo\": {  \n\"productCode\": \"HX_STD\",  \n\"deliveryDateOriginal\": \"2020-12-16T17:00:00.0000000+00:00\"  \n}  \n}\n\nRequest Sample4:  \n● Bugün Teslimattan Yarın Teslimat ya da Yarın Teslimattan Bugün Teslimat olarak güncellenen bir sipariş için aşağıdaki request ile gelinmesi gerekir.  \n● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.\n\n{  \n\"deliveryUpdateType\": \"PRODUCT_CODE\",  \n\"customerDeliveryNo\": \"3561217255F55\",  \n\"productCodeInfo\": {  \n\"productCode\": \"HX_ND\"  \n}  \n}\n\nRequest Sample5:  \n● Siparişteki alıcıya ait bilgileri güncellemek için aşağıdaki request ile gelinmelidir.  \n● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.\n\n{  \n\"deliveryUpdateType\": \"COMPANY_CUSTOMER\",  \n\"customerDeliveryNo\": \"3561217255F55\",  \n\"companyCustomerInfo\": {  \n\"companyCustomerId\": \"123444\",  \n\"firstName\": \"ali\",  \n\"lastName\": \"can\",  \n\"email\": \"ali@can\",  \n\"gsm1\": \"0536\"  \n}  \n}\n\nRequest Sample6:  \n● Siparişteki adres bilgisini güncellemek için aşağıdaki request ile gelinmelidir.  \n● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.\n\n{  \n\"deliveryUpdateType\": \"COMPANY_ADDRESS\",  \n\"customerDeliveryNo\": \"3561217255F55\",  \n\"companyAddressInfo\": {  \n\"companyAddressId\": \"123456\",  \n\"addressLine1\": \"kuşakçı sok no:1aaa5\",  \n\"countryName\": \"türkiye\",  \n\"cityName\": \"istnbulu\",  \n\"townName\": \"mallltepe\",  \n\"districtName\": \"adss\"  \n}  \n}\n\nRequest Sample7:  \n● Siparişteki alıcıya ait bilgileri güncellemek için aşağıdaki request ile gelinmelidir.  \n● Gönderi iş ortağı deposundan kabul etmeden ya da toplama yapılmadan güncellenmelidir.\n\n{  \n\"deliveryUpdateType\": \"RECIPIENT_PERSON\",  \n\"customerDeliveryNo\": \"6271087689840\",  \n\"recipientPersonInfo\": {  \n\"recipientPerson\": \"ali can\",  \n\"recipientPersonPhone1\": \"0536\",  \n\"recipientPersonPhone2\": \"0505\",  \n\"recipientPersonEmail\": \"ali@can\"  \n}  \n}",
        "requestBody": {
          "content": {
            "*/*": {
              "schema": {
                "type": "string",
                "example": "\"{\\r\\n \\\"deliveryUpdateType\\\": \\\"RECIPIENT_PERSON\\\",\\r\\n \\\"customerDeliveryNo\\\": \\\"HPT100000000\\\",\\r\\n \\\"recipientPersonInfo\\\": {\\r\\n   \\\"recipientPerson\\\": \\\"MEHMET TURAN\\\",\\r\\n   \\\"recipientPersonPhone1\\\": \\\"05322103019\\\"\\r\\n }\\r\\n}\\r\\n\""
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