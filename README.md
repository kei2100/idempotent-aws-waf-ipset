# idempotent-aws-waf-ipset

```go
import 	ipset "github.com/kei2100/idempotent-aws-waf-ipset"

func main() {
    ipset.AppendToIPSet(ctx, ipSetID, ipSetName, cidr)
    ipset.RemoveFromIPSet(ctx, ipSetID, ipSetName, cidr)
}
```
