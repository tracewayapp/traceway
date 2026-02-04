export interface User {
  id: number;
  firstName: string;
  lastName: string;
  email: string;
}

export interface CreateUserDto {
  firstName: string;
  lastName: string;
  email: string;
}

export interface UpdateUserDto {
  firstName?: string;
  lastName?: string;
  email?: string;
}
