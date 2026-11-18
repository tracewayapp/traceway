import { Controller, Get, Headers } from '@nestjs/common';
import { DashboardService } from './dashboard.service';

@Controller('api/dashboard')
export class DashboardController {
  constructor(private readonly dashboard: DashboardService) {}

  @Get()
  getDashboard(@Headers('x-account-id') accountId: string) {
    return this.dashboard.load(accountId);
  }
}
