import { Injectable, NotFoundException } from "@nestjs/common";
import { Span } from "@traceway/nestjs";
import { User, CreateUserDto, UpdateUserDto } from "./user.entity";

@Injectable()
export class UsersService {
  private users: User[] = [];
  private nextId = 1;

  @Span("db.users.findAll")
  async findAll(): Promise<User[]> {
    await this.simulateDbDelay();
    return [...this.users];
  }

  @Span("db.users.findOne")
  async findOne(id: number): Promise<User> {
    await this.simulateDbDelay();
    const user = this.users.find((u) => u.id === id);
    if (!user) {
      throw new NotFoundException(`User with id ${id} not found`);
    }
    return user;
  }

  @Span("db.users.create")
  async create(createUserDto: CreateUserDto): Promise<User> {
    await this.simulateDbDelay();
    const user: User = {
      id: this.nextId++,
      firstName: createUserDto.firstName,
      lastName: createUserDto.lastName,
      email: createUserDto.email,
    };
    this.users.push(user);
    return user;
  }

  @Span("db.users.update")
  async update(id: number, updateUserDto: UpdateUserDto): Promise<User> {
    await this.simulateDbDelay();
    const index = this.users.findIndex((u) => u.id === id);
    if (index === -1) {
      throw new NotFoundException(`User with id ${id} not found`);
    }
    const user = this.users[index];
    const updated = {
      ...user,
      ...updateUserDto,
    };
    this.users[index] = updated;
    return updated;
  }

  @Span("db.users.delete")
  async remove(id: number): Promise<{ message: string }> {
    await this.simulateDbDelay();
    const index = this.users.findIndex((u) => u.id === id);
    if (index === -1) {
      throw new NotFoundException(`User with id ${id} not found`);
    }
    this.users.splice(index, 1);
    return { message: "user deleted" };
  }

  private simulateDbDelay(): Promise<void> {
    return new Promise((resolve) =>
      setTimeout(resolve, 10 + Math.random() * 40),
    );
  }
}
