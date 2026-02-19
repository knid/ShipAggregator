<head><meta http-equiv="Content-Type" content="text/html; charset=ISO-8859-9" /></head>
<?php      

	#################################################################################################################################
	#																																#
	#  	 XML SERVISLERI PHP KULLANIM ORNEGI																							#
																								#
	#    GetQueryDs için yukarıda bulunan charset değeri Türkçe karakter sorunu yaşamamak için charset=UTF8 olarak değiştirmelidir. #
	#    Diğer methodlar içinse charset=ISO-8859-9 olarak kullanılmalıdır.															#
	#																																#
	#################################################################################################################################
	
	class arascargo {
	
	  #Değişken tanımlamaları burada yapılır
	  var $Servis;
	  var $DefaultEncoding = 'ISO-8859-9';
      var $Url = 'http://customerservices.araskargo.com.tr/ArasCargoCustomerIntegrationService/ArasCargoIntegrationService.svc?wsdl';
      var $UserName          = ''; 
      var $Password          = '';
	  var $CustomerCode      = '';
	  var $QueryType 	 	 = '2';
	  var $dtime 	 		 = '02.05.2013';	
	  var $data = array();
      var $Error = array();
	  
	  
	  #SOAP servisi için servis client'i burada oluşturulur
		function arascargo(){
			try {
				$return = $this->Servis = new SoapClient($this->Url, array('encoding'=>$this->DefaultEncoding)); 
            } catch(Exception $exp) {
				echo  $this->Error['construct'] = $exp->getMessage();
			}
		}
	  
		#GetQueryDS servisine baglanıp sorgulama yapan fonksiyon 
		function GetDataGetQueryDS(){
			#Servis 2 adet parametre alır. Bu parametreler burada tanımlanır ve değerleri atanır.
			$loginInfo         = '<LoginInfo><UserName>' . $this->UserName   . '</UserName><Password>'. $this->Password   .'</Password><CustomerCode>'. $this->CustomerCode   .'</CustomerCode></LoginInfo>';				
			$queryInfo         = '<QueryInfo><QueryType>'. $this->QueryType   .'</QueryType><date>'. $this->dtime   .'</date></QueryInfo>';
			try {
				$return = $this->Servis->GetQueryDS(array("loginInfo"=>$loginInfo,"queryInfo"=>$queryInfo));
				return $return;
			} catch(Exception $exp) {
				echo $this->Error['CreateShipment'] = $exp->getMessage();
			}
		}
		
		#GetQueryXML servisine baglanıp sorgulama yapan fonksiyon 
	    function GetDataGetQueryXML(){
	    #Servis 2 adet parametre alır. Bu parametreler burada tanımlanır ve değerleri atanır.
		$loginInfo         = '<LoginInfo><UserName>' . $this->UserName   . '</UserName><Password>'. $this->Password   .'</Password><CustomerCode>'. $this->CustomerCode   .'</CustomerCode></LoginInfo>';				
	    $queryInfo         = '<QueryInfo><QueryType>'. $this->QueryType   .'</QueryType><date>'. $this->dtime   .'</date></QueryInfo>';

		try {
				$return = $this->Servis->GetQueryXML(array("loginInfo"=>$loginInfo,"queryInfo"=>$queryInfo));
				return $return;
			} catch(Exception $exp) {
				echo $this->Error['CreateShipment'] = $exp->getMessage();
			}
		}
		
		#GetQueryJSON servisine baglanıp sorgulama yapan fonksiyon 
	    function GetDataGetQueryJSON(){
			#Servis 2 adet parametre alır. Bu parametreler burada tanımlanır ve değerleri atanır.
			$loginInfo         = '<LoginInfo><UserName>' . $this->UserName   . '</UserName><Password>'. $this->Password   .'</Password><CustomerCode>'. $this->CustomerCode   .'</CustomerCode></LoginInfo>';				
			$queryInfo         = '<QueryInfo><QueryType>'. $this->QueryType   .'</QueryType><date>'. $this->dtime   .'</date></QueryInfo>';

			try {
				$return = $this->Servis->GetQueryJSON(array("loginInfo"=>$loginInfo,"queryInfo"=>$queryInfo));
				return $return;
			} catch(Exception $exp) {
				echo $this->Error['CreateShipment'] = $exp->getMessage();
            }
        }
	}
	
	# GetQueryDS servisi burada çağırılıyor.
	$aras = new arascargo();  
	$sonuc = $aras->GetDataGetQueryDS();
	ResponseArray($sonuc);	 
	 
	#GetQueryXML servisi burada çağırılıyor
	$sonuc = $aras->GetDataGetQueryXML();
	ResponseArray($sonuc);
	 
	#GetQueryJSON servisi burada çağırılıyor
	$sonuc = $aras->GetDataGetQueryJSON();
	ResponseArray($sonuc);
	   
	#Dönen sonuçları ekrana yazdırır
	function ResponseArray($array){
		echo '<pre>';
        print_r($array);
        echo '</pre>';
	}
?>