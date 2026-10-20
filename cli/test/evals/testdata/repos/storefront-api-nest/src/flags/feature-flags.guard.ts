import { CanActivate, ExecutionContext, Injectable } from '@nestjs/common';
import { FeatureFlagsService } from './feature-flags.service';

@Injectable()
export class FeatureFlagsGuard implements CanActivate {
  constructor(private readonly flags: FeatureFlagsService) {}

  async canActivate(ctx: ExecutionContext): Promise<boolean> {
    const req = ctx.switchToHttp().getRequest();
    req.flags = await this.flags.load();
    if (req.flags.get('storefront.maintenance')?.enabled && !req.path.startsWith('/api/status')) {
      return false;
    }
    return true;
  }
}
