import { CanActivate, ExecutionContext, Injectable, ServiceUnavailableException } from '@nestjs/common';
import { FeatureFlagsService } from './feature-flags.service';

@Injectable()
export class FeatureFlagsGuard implements CanActivate {
  constructor(private readonly flags: FeatureFlagsService) {}

  async canActivate(ctx: ExecutionContext): Promise<boolean> {
    const req = ctx.switchToHttp().getRequest();
    req.flags = await this.flags.load();
    if (req.flags.get('storefront.maintenance')?.enabled) {
      throw new ServiceUnavailableException('storefront is in maintenance');
    }
    return true;
  }
}
