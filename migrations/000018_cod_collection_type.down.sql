ALTER TABLE shipments DROP COLUMN IF EXISTS cod_installments;
ALTER TABLE shipments DROP COLUMN IF EXISTS cod_collection_type;

ALTER TABLE quotes DROP COLUMN IF EXISTS cod_installments;
ALTER TABLE quotes DROP COLUMN IF EXISTS cod_collection_type;
