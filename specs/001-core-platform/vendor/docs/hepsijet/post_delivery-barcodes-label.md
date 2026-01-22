# /BarcodeLabel

Bu servisin kullanım amacı Barcode'ın ZPL format dışında başka bir formata dönüştürülmesini sağlamaktır.

barcodes-label?format=PDF servis kullanımı

Method: POST

Örnek Request Body:

{

"barcodes": \[

"35731697506",

"35731697506",

\]

}

Format kısmında PDF, JPEG, PNG şeklinde  
çağırılabilir. (URL: format=PDF, format=JPEG, format=PNG)

PDF olarak çağırıldığında, bütün barkodlar  
tek bir PDF içerisine sayfa sayfa yerleştirilir.

PNG ve JPEG ler ise bir liste halinde  
döner.

Her response Base64 encode edilmiş şekilde  
döner. İçeriği görebilmek için, Base64 kodlanmış response’u decode etmek  
gerekir.

Decode işlemi için örnek web sitesi: [https://base64.guru/converter/decode/pdf](https://base64.guru/converter/decode/pdf)

Base64 to PDF | Base64 Decode | Base64  
Converter | Base64

Convert Base64 to PDF online using a free  
decoding tool

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
    "/delivery/barcodes-label": {
      "post": {
        "tags": [
          "Hepsijet"
        ],
        "summary": "/BarcodeLabel",
        "description": "Bu servisin kullanım amacı Barcode'ın ZPL format dışında başka bir formata dönüştürülmesini sağlamaktır.\n\nbarcodes-label?format=PDF servis kullanımı\n\nMethod: POST\n\nÖrnek Request Body:\n\n{\n\n\"barcodes\": \\[\n\n\"35731697506\",\n\n\"35731697506\",\n\n\\]\n\n}\n\nFormat kısmında PDF, JPEG, PNG şeklinde  \nçağırılabilir. (URL: format=PDF, format=JPEG, format=PNG)\n\nPDF olarak çağırıldığında, bütün barkodlar  \ntek bir PDF içerisine sayfa sayfa yerleştirilir.\n\nPNG ve JPEG ler ise bir liste halinde  \ndöner.\n\nHer response Base64 encode edilmiş şekilde  \ndöner. İçeriği görebilmek için, Base64 kodlanmış response’u decode etmek  \ngerekir.\n\nDecode işlemi için örnek web sitesi: [https://base64.guru/converter/decode/pdf](https://base64.guru/converter/decode/pdf)\n\nBase64 to PDF | Base64 Decode | Base64  \nConverter | Base64\n\nConvert Base64 to PDF online using a free  \ndecoding tool",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "example": {
                  "barcodes": [
                    "140641960"
                  ]
                }
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
            "name": "format",
            "in": "query",
            "schema": {
              "type": "string"
            },
            "example": "PDF"
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