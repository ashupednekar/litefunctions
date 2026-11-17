package adaptors

import (
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func FunctionFromCtx(c *gin.Context, pool DBTX) (*Function, error){
	fnHex := c.Param("fnID")
	fnID, err := hex.DecodeString(fnHex)
	if err != nil || len(fnID) != 16 {
		return nil, err
	}
	pgFnId := pgtype.UUID{Valid: true}
	copy(pgFnId.Bytes[:], fnID)
	q := New(pool)
	f, err := q.GetFunctionByID(c.Request.Context(), pgFnId)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
