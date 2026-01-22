-- Documents CRUD queries
-- Table: documents

-- name: GetDocumentByID :one
SELECT id, organization_id, type, format, filename, storage_path, storage_bucket,
       content_type, size_bytes, content_hash, metadata, created_at
FROM documents
WHERE id = $1;

-- name: GetDocumentByContentHash :one
SELECT id, organization_id, type, format, filename, storage_path, storage_bucket,
       content_type, size_bytes, content_hash, metadata, created_at
FROM documents
WHERE organization_id = $1 AND content_hash = $2
LIMIT 1;

-- name: ListDocumentsByOrganization :many
SELECT id, organization_id, type, format, filename, storage_path, storage_bucket,
       content_type, size_bytes, content_hash, metadata, created_at
FROM documents
WHERE organization_id = $1
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type')::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountDocumentsByOrganization :one
SELECT COUNT(*) as total
FROM documents
WHERE organization_id = $1
  AND (sqlc.narg('type')::varchar IS NULL OR type = sqlc.narg('type')::varchar);

-- name: CreateDocument :one
INSERT INTO documents (
    organization_id,
    type,
    format,
    filename,
    storage_path,
    storage_bucket,
    content_type,
    size_bytes,
    content_hash,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, organization_id, type, format, filename, storage_path, storage_bucket,
          content_type, size_bytes, content_hash, metadata, created_at;

-- name: DeleteDocument :exec
DELETE FROM documents
WHERE id = $1;

-- name: ListDocumentsByIDs :many
SELECT id, organization_id, type, format, filename, storage_path, storage_bucket,
       content_type, size_bytes, content_hash, metadata, created_at
FROM documents
WHERE id = ANY($1::uuid[]);

-- name: GetDocumentStorageStats :one
SELECT 
    COUNT(*) as total_documents,
    COALESCE(SUM(size_bytes), 0) as total_size_bytes
FROM documents
WHERE organization_id = $1;

-- name: ListOrphanedDocuments :many
SELECT d.id, d.organization_id, d.type, d.format, d.filename, d.storage_path, 
       d.storage_bucket, d.content_type, d.size_bytes, d.content_hash, d.metadata, d.created_at
FROM documents d
LEFT JOIN shipments s ON d.id = s.label_document_id
WHERE d.type = 'label' AND s.id IS NULL AND d.created_at < NOW() - INTERVAL '7 days'
LIMIT sqlc.arg('limit_count')::int;
