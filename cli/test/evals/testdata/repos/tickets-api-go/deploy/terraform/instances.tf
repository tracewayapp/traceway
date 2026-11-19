resource "aws_instance" "tickets" {
  for_each = toset(["a", "b", "c"])

  ami                    = data.aws_ami.tickets.id
  instance_type          = "m6i.large"
  subnet_id              = aws_subnet.private[each.key].id
  vpc_security_group_ids = [aws_security_group.tickets.id]
  iam_instance_profile   = aws_iam_instance_profile.tickets.name
  user_data              = templatefile("${path.module}/cloud-init.yaml", { hostname = "tickets-node-${each.key}" })

  tags = {
    Name    = "tickets-node-${each.key}"
    Service = "tickets-api"
  }
}

resource "aws_lb_target_group_attachment" "tickets" {
  for_each         = aws_instance.tickets
  target_group_arn = aws_lb_target_group.tickets.arn
  target_id        = each.value.id
  port             = 8080
}

resource "aws_lb_target_group" "tickets" {
  name                          = "tickets-api"
  port                          = 8080
  protocol                      = "HTTP"
  vpc_id                        = aws_vpc.main.id
  load_balancing_algorithm_type = "round_robin"

  health_check {
    path     = "/healthz"
    interval = 15
  }
}
