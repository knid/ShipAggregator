-- Add COD collection type and installments columns to quotes and shipments
ALTER TABLE quotes ADD COLUMN cod_collection_type TEXT NULL;
ALTER TABLE quotes ADD COLUMN cod_installments INTEGER NULL;

ALTER TABLE shipments ADD COLUMN cod_collection_type TEXT NULL;
ALTER TABLE shipments ADD COLUMN cod_installments INTEGER NULL;
